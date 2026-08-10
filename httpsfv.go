// Package httpsfv implements parsing of HTTP Structured Field Values as specified in RFC 9651.
package httpsfv

import (
	"encoding"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
	"unsafe"

	"github.com/nussjustin/httpsfv/ordered"
)

var (
	// ErrInvalidBareItem is returned when parsing a bare item fails.
	ErrInvalidBareItem = errors.New("invalid bare item")

	// ErrInvalidBoolean is returned when parsing a boolean fails.
	ErrInvalidBoolean = errors.New("invalid boolean")

	// ErrInvalidByteSequence is returned when parsing a byte sequence fails.
	ErrInvalidByteSequence = errors.New("invalid byte sequence")

	// ErrInvalidDate is returned when parsing a date fails.
	ErrInvalidDate = errors.New("invalid date")

	// ErrInvalidDictionary is returned when parsing a dictionary fails.
	ErrInvalidDictionary = errors.New("invalid dictionary")

	// ErrInvalidDisplayString is returned when parsing a display string fails.
	ErrInvalidDisplayString = errors.New("invalid display string")

	// ErrInvalidInnerList is returned when parsing an inner list fails.
	ErrInvalidInnerList = errors.New("invalid inner list")

	// ErrInvalidIntegerOrDecimal is returned when parsing an integer or decimal fails.
	ErrInvalidIntegerOrDecimal = errors.New("invalid integer or decimal")

	// ErrInvalidItem is returned when parsing an item fails.
	ErrInvalidItem = errors.New("invalid item")

	// ErrInvalidKey is returned when parsing a key fails.
	ErrInvalidKey = errors.New("invalid key")

	// ErrInvalidList is returned when parsing a list fails.
	ErrInvalidList = errors.New("invalid list")

	// ErrInvalidParameters is returned when parsing parameters fails.
	ErrInvalidParameters = errors.New("invalid parameters")

	// ErrInvalidString is returned when parsing a string fails.
	ErrInvalidString = errors.New("invalid string")

	// ErrInvalidToken is returned when parsing a token fails.
	ErrInvalidToken = errors.New("invalid token")

	// ErrNonAsciiInput is returned when parsing a structured field containing non-ASCII characters.
	ErrNonAsciiInput = errors.New("non-ascii input")

	// ErrTrailingData is returned when parsing a structured field containing trailing data.
	ErrTrailingData = errors.New("trailing data")
)

type prefixedError struct {
	prefix error
	err    error
}

func prefixError(prefix, err error) error {
	return &prefixedError{prefix, err}
}

// Error implements the [error] interface.
func (p *prefixedError) Error() string {
	return fmt.Sprintf("%s: %s", p.prefix, p.err)
}

func (p *prefixedError) Unwrap() []error {
	return []error{p.prefix, p.err}
}

// Parse parses the given string and returns as either [Dictionary], [Item] or [List] based on the generic type
// parameter T, as specified in RFC 9651 section 4.2.
//
// An empty input returns a zero value and a nil error.
func Parse[T Dictionary | Item | List](inputString string) (T, error) {
	if len(inputString) == 0 {
		var zero T
		return zero, nil
	}

	// From https://www.rfc-editor.org/info/rfc9651/#name-parsing-structured-fields
	//
	// 1. Convert input_bytes into an ASCII string input_string; if conversion fails, fail parsing.
	//
	// Note: We take a string, so we do not need to convert anything, but we must still check if the string contains only ASCII.
	if !isASCII(inputString) {
		var zero T
		return zero, ErrNonAsciiInput
	}

	// 2. Discard any leading SP characters from input_string.
	inputString = trimLeadingSP(inputString)

	var result T
	var err error

	switch any(result).(type) {
	case List:
		var v List
		// 3. If field_type is "list", let output be the result of running Parsing a List (Section 4.2.1) with input_string.
		v, inputString, err = parseList(inputString)
		result = *(*T)(unsafe.Pointer(&v))
	case Dictionary:
		var v Dictionary
		// 4. If field_type is "dictionary", let output be the result of running Parsing a Dictionary (Section 4.2.2) with input_string.
		v, inputString, err = parseDictionary(inputString)
		result = *(*T)(unsafe.Pointer(&v))
	case Item:
		var v Item
		// 5. If field_type is "item", let output be the result of running Parsing an Item (Section 4.2.3) with input_string.
		v, inputString, err = parseItem(inputString)
		result = *(*T)(unsafe.Pointer(&v))
	}

	if err != nil {
		var zero T
		return zero, err
	}

	// 6. Discard any leading SP characters from input_string.
	inputString = trimLeadingSP(inputString)

	// 7. If input_string is not empty, fail parsing.
	if inputString != "" {
		var zero T
		return zero, fmt.Errorf("%w: %q", ErrTrailingData, inputString)
	}

	// 8. Otherwise, return output.
	return result, nil
}

// ParseLines is like [Parse] but validates takes a list of inputs, generally each specified header line for the given
// header, and validates that each string can be parsed without the other strings.
//
// If any input string cannot be parsed, an error is returned.
//
// Otherwise, the result of parsing the strings as one, joined by a comma and a space, is returned.
func ParseLines[T Dictionary | Item | List](inputStrings []string) (T, error) {
	if len(inputStrings) == 0 {
		var zero T
		return zero, nil
	}

	// Fast-path: Avoid parsing the input twice
	if len(inputStrings) == 1 {
		return Parse[T](inputStrings[0])
	}

	for _, inputString := range inputStrings {
		if v, err := Parse[T](inputString); err != nil {
			return v, err
		}
	}

	return Parse[T](strings.Join(inputStrings, ", "))
}

func decodeHex(b byte) byte {
	if b >= 'a' && b <= 'f' {
		return (b - 'a') + 10
	}

	if b >= '0' && b <= '9' {
		return b - '0'
	}

	panic("unreachable")
}

func encodeHex(b byte) (e1 byte, e2 byte) {
	const hexTable = "0123456789abcdef"
	e1 = hexTable[b>>4]
	e2 = hexTable[b&0x0f]
	return
}

func isASCII(s string) bool {
	for _, c := range []byte(s) {
		if c > 127 {
			return false
		}
	}
	return true
}

// Constants for https://www.rfc-editor.org/info/rfc5234
const (
	_DQUOTE = byte(0x22)
	_HTAB   = byte(0x09)
	_SP     = byte(0x20)
)

func isLcalpha(b byte) bool {
	return b >= 0x61 && b <= 0x7A
}

func isRFC5234ALPHA(b byte) bool {
	return (b >= 0x41 && b <= 0x5A) || (b >= 0x61 && b <= 0x7A)
}

func isRFC5234DIGIT(b byte) bool {
	return b >= 0x30 && b <= 0x39
}

func isRFC5234VCHAR(b byte) bool {
	return b >= 0x21 && b <= 0x7e
}

func isRFC9110tchar(b byte) bool {
	switch b {
	case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
		return true
	}
	if isRFC5234ALPHA(b) || isRFC5234DIGIT(b) {
		return true
	}
	if isRFC5234VCHAR(b) {
		switch b {
		case _DQUOTE, '(', ')', ',', '/', ':', ';', '<', '=', '>', '?', '@', '[', '\\', ']', '{', '}':
			return false
		default:
			return true
		}
	}
	return false
}

func trimLeadingOWS(s string) string {
	for i, c := range []byte(s) {
		if c == _HTAB || c == _SP {
			continue
		}

		return s[i:]
	}

	return ""
}

func trimLeadingSP(s string) string {
	for i, c := range []byte(s) {
		if c == _SP {
			continue
		}

		return s[i:]
	}

	return ""
}

// parseKey parses a key as specified in RFC 9651 section 4.2.3.3 "Parsing a Key".
func parseKey(inputString string) (key string, rest string, err error) {
	// From https://www.rfc-editor.org/info/rfc9651/#name-parsing-a-key
	//
	// 1. If the first character of input_string is not lcalpha or "*", fail parsing.

	if inputString == "" {
		return "", "", ErrInvalidKey
	}

	if first := inputString[0]; !isLcalpha(first) && first != '*' {
		return "", "", ErrInvalidKey
	}

	// 2. Let output_string be an empty string.
	// -> Replaced with index to avoid allocations
	var curr int

	// 3. While input_string is not empty:
	for inputString[curr:] != "" {
		// 1. If the first character of input_string is not one of lcalpha, DIGIT, "_", "-", ".", or "*", return output_string.
		switch {
		case isLcalpha(inputString[curr]):
		case isRFC5234DIGIT(inputString[curr]):
		case inputString[curr] == '_':
		case inputString[curr] == '-':
		case inputString[curr] == '.':
		case inputString[curr] == '*':
		default:
			return inputString[:curr], inputString[curr:], nil
		}

		// 2. Let char be the result of consuming the first character of input_string.
		// -> Optimized away

		// 3. Append char to output_string.
		curr++
	}

	// 4. Return output_string.
	return inputString[:curr], inputString[curr:], nil
}

// serializeKey serializes a key as specified in RFC 9651 section 4.1.1.3 "Serializing a Key".
func serializeKey(dst []byte, inputKey string) ([]byte, error) {
	// From https://www.rfc-editor.org/info/rfc9651/#ser-key
	//
	// 1. Convert input_key into a sequence of ASCII characters; if conversion fails, fail serialization.
	if !isASCII(inputKey) {
		return nil, prefixError(ErrInvalidKey, ErrNonAsciiInput)
	}

	// 2. If input_key contains characters not in lcalpha, DIGIT, "_", "-", ".", or "*", fail serialization.
	for _, char := range []byte(inputKey) {
		switch {
		case isLcalpha(char),
			isRFC5234DIGIT(char),
			char == '_',
			char == '-',
			char == '.',
			char == '*':
		default:
			return nil, fmt.Errorf("%w: invalid character", ErrInvalidKey)
		}
	}

	// 3. If the first character of input_key is not lcalpha or "*", fail serialization.
	if inputKey == "" || (!isLcalpha(inputKey[0]) && inputKey[0] != '*') {
		return nil, fmt.Errorf("%w: invalid or missing prefix character", ErrInvalidKey)
	}

	// 4. Let output be an empty string.
	output := dst

	// 5. Append input_key to output.
	output = append(output, inputKey...)

	// 6. Return output.
	return output, nil
}

// BareItem represents a simple item without parameters.
//
// It acts as a tagged union, with [BareItem.Type] returning the type of the item.
type BareItem struct {
	typ BareItemType

	bytes []byte
	num   uint64
	str   string
}

// BareItemType is an enum of types for a BareItem.
type BareItemType uint8

const (
	// BareItemTypeInvalid is the zero value of [BareItemType] and not a valid value.
	BareItemTypeInvalid BareItemType = iota

	// BareItemTypeBoolean denotes a boolean value as specified in RFC 9651 section 3.3.6.
	BareItemTypeBoolean

	// BareItemTypeByteSequence denotes a byte sequence as specified in RFC 9651 section 3.3.5.
	BareItemTypeByteSequence

	// BareItemTypeDate denotes a date as specified in RFC 9651 section 3.3.7.
	BareItemTypeDate

	// BareItemTypeDecimal denotes a decimal value as specified in RFC 9651 section 3.3.3.
	BareItemTypeDecimal

	// BareItemTypeDisplayString denotes a display string as specified in RFC 9651 section 3.3.8.
	BareItemTypeDisplayString

	// BareItemTypeInteger denotes an integer value as specified in RFC 9651 section 3.3.1.
	BareItemTypeInteger

	// BareItemTypeString denotes a string as specified in RFC 9651 section 3.3.3.
	BareItemTypeString

	// BareItemTypeToken denotes a token as specified in RFC 9651 section 3.3.4.
	BareItemTypeToken
)

// parseBareItem parses a single bare item as specified in RFC 9651 section 4.3.2.1 "Parsing an Item".
func parseBareItem(inputString string) (v BareItem, rest string, err error) {
	if len(inputString) == 0 {
		return BareItem{}, "", ErrInvalidBareItem
	}

	first := inputString[0]

	// From https://www.rfc-editor.org/info/rfc9651/#name-parsing-a-bare-item
	switch {
	case first == '-' || isRFC5234DIGIT(first):
		// 1. If the first character of input_string is a "-" or a DIGIT, return the result of running Parsing an Integer or Decimal (Section 4.2.4) with input_string.
		return parseBareItemIntegerOrDecimal(inputString)
	case first == _DQUOTE:
		// 2. If the first character of input_string is a DQUOTE, return the result of running Parsing a String (Section 4.2.5) with input_string.
		v, rest, err = parseBareItemString(inputString)
	case isRFC5234ALPHA(first) || first == '*':
		// 3. If the first character of input_string is an ALPHA or "*", return the result of running Parsing a Token (Section 4.2.6) with input_string
		return parseBareItemToken(inputString)
	case first == ':':
		// 4. If the first character of input_string is ":", return the result of running Parsing a Byte Sequence (Section 4.2.7) with input_string.
		v, rest, err = parseBareItemByteSequence(inputString)
	case first == '?':
		// 5. If the first character of input_string is "?", return the result of running Parsing a Boolean (Section 4.2.8) with input_string.
		v, rest, err = parseBareItemBoolean(inputString)
	case first == '@':
		// 6. If the first character of input_string is "@", return the result of running Parsing a Date (Section 4.2.9) with input_string.
		v, rest, err = parseBareItemDate(inputString)
	case first == '%':
		// 7. If the first character of input_string is "%", return the result of running Parsing a Display String (Section 4.2.10) with input_string.
		v, rest, err = parseBareItemDisplayString(inputString)
	default:
		// 8. Otherwise, the item type is unrecognized; fail parsing
		return BareItem{}, "", ErrInvalidBareItem
	}

	if err != nil {
		err = prefixError(ErrInvalidBareItem, err)
	}

	return
}

func serializeBareItem(dst []byte, b BareItem) ([]byte, error) {
	switch b.Type() {
	case BareItemTypeInteger:
		return serializeBareItemInteger(dst, b.Integer())
	case BareItemTypeDecimal:
		return serializeBareItemDecimal(dst, b.Decimal())
	case BareItemTypeString:
		return serializeBareItemString(dst, b.String())
	case BareItemTypeToken:
		return serializeBareItemToken(dst, b.Token())
	case BareItemTypeByteSequence:
		return serializeBareItemByteSequence(dst, b.ByteSequence())
	case BareItemTypeBoolean:
		return serializeBareItemBoolean(dst, b.Boolean())
	case BareItemTypeDate:
		return serializeBareItemDate(dst, b.Date())
	case BareItemTypeDisplayString:
		return serializeBareItemDisplayString(dst, b.DisplayString())
	default:
		panic(fmt.Sprintf("unknown BareItem type: %d", b.typ))
	}
}

// BareItemInteger returns a BareItem holding the given integer value.
func BareItemInteger(i int64) BareItem {
	return BareItem{typ: BareItemTypeInteger, num: uint64(i)}
}

// Integer returns the underlying integer value.
//
// It panics if b is not of type [BareItemTypeInteger].
func (b *BareItem) Integer() int64 {
	if b.typ != BareItemTypeInteger {
		panic("BareItem is not an integer")
	}
	return int64(b.num)
}

// BareItemDecimal returns a BareItem holding the given decimal value.
//
// No validation is performed on the value, and serializing the value may fail if it is not valid.
func BareItemDecimal(d float64) BareItem {
	return BareItem{typ: BareItemTypeDecimal, num: math.Float64bits(d)}
}

// Decimal returns the underlying decimal value.
//
// It panics if b is not of type [BareItemTypeDecimal].
func (b *BareItem) Decimal() float64 {
	if b.typ != BareItemTypeDecimal {
		panic("BareItem is not a decimal")
	}
	return math.Float64frombits(b.num)
}

// BareItemString returns a BareItem holding the given string value.
//
// No validation is performed on the value, and serializing the value may fail if it is not valid.
func BareItemString(s string) BareItem {
	return BareItem{typ: BareItemTypeString, str: s}
}

// String returns the underlying string value.
//
// It panics if b is not of type [BareItemTypeString].
func (b *BareItem) String() string {
	if b.typ != BareItemTypeString {
		panic("BareItem is not a string")
	}
	return b.str
}

// BareItemToken returns a BareItem holding the given token value.
//
// No validation is performed on the value, and serializing the value may fail if it is not valid.
func BareItemToken(t string) BareItem {
	return BareItem{typ: BareItemTypeToken, str: t}
}

// Token returns the underlying token value.
//
// It panics if b is not of type [BareItemTypeToken].
func (b *BareItem) Token() string {
	if b.typ != BareItemTypeToken {
		panic("BareItem is not a token")
	}
	return b.str
}

// BareItemByteSequence returns a BareItem holding the given byte sequence value.
//
// No validation is performed on the value, and serializing the value may fail if it is not valid.
//
// Note that bs is not copied. Modifications to the underlying value will affect the returned bare item.
func BareItemByteSequence(bs []byte) BareItem {
	return BareItem{typ: BareItemTypeByteSequence, bytes: bs}
}

// ByteSequence returns the underlying byte sequence value.
//
// It panics if b is not of type [BareItemTypeByteSequence].
func (b *BareItem) ByteSequence() []byte {
	if b.typ != BareItemTypeByteSequence {
		panic("BareItem is not a byte sequence")
	}
	return b.bytes
}

// BareItemBoolean returns a BareItem holding the given boolean value.
//
// No validation is performed on the value, and serializing the value may fail if it is not valid.
func BareItemBoolean(b bool) BareItem {
	if b {
		return BareItem{typ: BareItemTypeBoolean, num: 1}
	}
	return BareItem{typ: BareItemTypeBoolean, num: 0}
}

// Boolean returns the underlying boolean value.
//
// It panics if b is not of type [BareItemTypeBoolean].
func (b *BareItem) Boolean() bool {
	if b.typ != BareItemTypeBoolean {
		panic("BareItem is not a boolean")
	}
	return b.num == 1
}

// BareItemDate returns a BareItem holding the given date value.
//
// No validation is performed on the value, and serializing the value may fail if it is not valid.
func BareItemDate(d int64) BareItem {
	return BareItem{typ: BareItemTypeDate, num: uint64(d)}
}

// Date returns the underlying date value.
//
// It panics if b is not of type [BareItemTypeDate].
func (b *BareItem) Date() int64 {
	if b.typ != BareItemTypeDate {
		panic("BareItem is not a date")
	}
	return int64(b.num)
}

// BareItemDisplayString returns a BareItem holding the given display string value.
//
// No validation is performed on the value, and serializing the value may fail if it is not valid.
func BareItemDisplayString(d string) BareItem {
	return BareItem{typ: BareItemTypeDisplayString, str: d}
}

// DisplayString returns the underlying display string value.
//
// It panics if b is not of type [BareItemTypeDate].
func (b *BareItem) DisplayString() string {
	if b.typ != BareItemTypeDisplayString {
		panic("BareItem is not a display string")
	}
	return b.str
}

// parseBareItemIntegerOrDecimal parses an integer or decimal as specified in RFC 9651 section 4.3.4 "Parsing an Integer or Decimal".
func parseBareItemIntegerOrDecimal(inputString string) (v BareItem, rest string, err error) {
	// From https://www.rfc-editor.org/info/rfc9651/#name-parsing-an-integer-or-decim
	//
	// 1. Let type be "integer".
	type_ := BareItemTypeInteger

	// 2. Let sign be 1.
	sign := 1

	// 3. Let input_number be an empty string.
	// -> Replaced with index to avoid allocations
	var curr int

	// 4. If the first character of input_string is "-", consume it and set sign to -1.
	if inputString != "" && inputString[0] == '-' {
		sign, inputString = -1, inputString[1:]
	}

	if inputString == "" {
		return BareItem{}, "", ErrInvalidIntegerOrDecimal
	}

	// 6. If the first character of input_string is not a DIGIT, fail parsing.
	if !isRFC5234DIGIT(inputString[0]) {
		return BareItem{}, "", ErrInvalidIntegerOrDecimal
	}

	// 7. While input_string is not empty:
	for curr < len(inputString) {
		// 1. Let char be the result of consuming the first character of input_string.
		// -> Consuming optimized away
		char := inputString[curr]

		// 2. If char is a DIGIT, append it to input_number.
		if isRFC5234DIGIT(char) {
			curr++
			continue
		}

		// 3. Else, if type is "integer" and char is ".":
		if type_ == BareItemTypeInteger && char == '.' {
			// 1. If input_number contains more than 12 characters, fail parsing.
			if curr > 12 {
				return BareItem{}, "", fmt.Errorf("%w: integer part too long", ErrInvalidIntegerOrDecimal)
			}

			// 2. Otherwise, append char to input_number and set type to "decimal".
			curr++
			type_ = BareItemTypeDecimal
			continue
		}

		// 4. Otherwise, prepend char to input_string, and exit the loop.
		break
	}

	inputNumber, inputString := inputString[:curr], inputString[curr:]

	// 7.5. If type is "integer" and input_number contains more than 15 characters, fail parsing.
	if type_ == BareItemTypeInteger && len(inputNumber) > 15 {
		return BareItem{}, "", fmt.Errorf("%w: integer too long", ErrInvalidIntegerOrDecimal)
	}

	// 7.6. If type is "decimal" and input_number contains more than 16 characters, fail parsing.
	if type_ == BareItemTypeDecimal && len(inputNumber) > 16 {
		return BareItem{}, "", fmt.Errorf("%w: decimal too long", ErrInvalidIntegerOrDecimal)
	}

	var outputNumberInt int64
	var outputNumberFloat float64

	// 8. If type is "integer":
	if type_ == BareItemTypeInteger {
		// 1. Let output_number be an Integer that is the result of parsing input_number as an integer.
		outputNumberInt, _ = strconv.ParseInt(inputNumber, 10, 64)
	}

	// 9. Otherwise:
	if type_ != BareItemTypeInteger {
		// 1. If the final character of input_number is ".", fail parsing.
		if inputNumber[len(inputNumber)-1] == '.' {
			return BareItem{}, "", fmt.Errorf("%w: missing decimal part", ErrInvalidIntegerOrDecimal)
		}

		// 2. If the number of characters after "." in input_number is greater than three, fail parsing.
		if dotPos := strings.IndexByte(inputNumber, '.'); len(inputNumber)-dotPos+-1 > 3 {
			return BareItem{}, "", fmt.Errorf("%w: too many digits in decimal part", ErrInvalidIntegerOrDecimal)
		}

		// 3. Let output_number be a Decimal that is the result of parsing input_number as a decimal number.
		outputNumberFloat, _ = strconv.ParseFloat(inputNumber, 64)
	}

	// 10. Let output_number be the product of output_number and sign.
	outputNumberInt *= int64(sign)
	outputNumberFloat *= float64(sign)

	// 11. Return output_number.
	if type_ == BareItemTypeInteger {
		return BareItemInteger(outputNumberInt), inputString, nil
	}

	return BareItemDecimal(outputNumberFloat), inputString, nil
}

// serializeBareItemInteger serializes an integer as specified in RFC 9651 section 4.1.4 "Serializing an Integer".
func serializeBareItemInteger(dst []byte, inputInteger int64) ([]byte, error) {
	// From https://www.rfc-editor.org/info/rfc9651/#section-4.1.4
	//
	// 1. If input_integer is not an integer in the range of -999,999,999,999,999 to 999,999,999,999,999 inclusive, fail
	//    serialization.
	if inputInteger < -999_999_999_999_999 || inputInteger > 999_999_999_999_999 {
		return nil, fmt.Errorf("%w: integer out of range", ErrInvalidIntegerOrDecimal)
	}

	// 2. Let output be an empty string.
	output := dst

	// 3. If input_integer is less than (but not equal to) 0, append "-" to output.
	// 4. Append input_integer's numeric value represented in base 10 using only decimal digits to output.
	output = strconv.AppendInt(output, inputInteger, 10)

	// 5. Return output.
	return output, nil
}

// serializeBareItemDecimal serializes a decimal as specified in RFC 9651 section 4.1.5 "Serializing a Decimal".
func serializeBareItemDecimal(dst []byte, inputDecimal float64) ([]byte, error) {
	// From https://www.rfc-editor.org/info/rfc9651/#section-4.1.5
	//
	// 1. If input_decimal is not a decimal number, fail serialization.
	if math.IsNaN(inputDecimal) {
		return nil, fmt.Errorf("%w: NaN", ErrInvalidIntegerOrDecimal)
	}

	// 2. If input_decimal has more than three significant digits to the right of the decimal point, round it to three
	//    decimal places, rounding the final digit to the nearest value, or to the even value if it is equidistant.
	inputDecimal = math.Round(inputDecimal*1_000) / 1_000

	// 3. If input_decimal has more than 12 significant digits to the left of the decimal point after rounding, fail serialization.
	if inputDecimal < -999_999_999_999 || inputDecimal > 999_999_999_999 {
		return nil, fmt.Errorf("%w: decimal out of range", ErrInvalidIntegerOrDecimal)
	}

	// 4. Let output be an empty string.
	output := dst

	hasFractional := inputDecimal != math.Trunc(inputDecimal)

	// 5. If input_decimal is less than (but not equal to) 0, append "-" to output.
	// 6. Append input_decimal's integer component represented in base 10 (using only decimal digits) to output; if it is zero, append "0".
	// 7. Append "." to output.
	// 8. If input_decimal's fractional component is zero, append "0" to output.
	// 9. Otherwise, append the significant digits of input_decimal's fractional component represented in base 10 (using only decimal digits) to output.
	if hasFractional {
		output = strconv.AppendFloat(output, inputDecimal, 'f', -1, 64)
	} else {
		output = strconv.AppendFloat(output, inputDecimal, 'f', 1, 64)
	}

	// 10. Return output.
	return output, nil
}

// parseBareItemString parses a string as specified in RFC 9651 section 4.2.5 "Parsing a String".
func parseBareItemString(inputString string) (v BareItem, rest string, err error) {
	// From https://www.rfc-editor.org/info/rfc9651/#section-4.2.5
	//
	// 1. Let output_string be an empty string.
	// -> Replaced with index to avoid allocations
	var curr int

	// 2. If the first character of input_string is not DQUOTE, fail parsing.
	if inputString == "" || inputString[0] != _DQUOTE {
		return BareItem{}, "", fmt.Errorf("%w: invalid or missing prefix character", ErrInvalidString)
	}

	// 3. Discard the first character of input_string.
	curr++

	var outputString strings.Builder
	outputStringEnd := 1

	// 4. While input_string is not empty:
	for curr < len(inputString) {
		// 1. Let char be the result of consuming the first character of input_string.
		char := inputString[curr]
		curr++

		// 2. If char is a backslash ("\"):
		if char == '\\' {
			// 1. If input_string is now empty, fail parsing.
			if curr >= len(inputString) {
				return BareItem{}, "", fmt.Errorf("%w: invalid escape sequence", ErrInvalidString)
			}

			// 2. Let next_char be the result of consuming the first character of input_string.
			// -> Consuming optimized away
			nextChar := inputString[curr]
			curr++

			// 3. If next_char is not DQUOTE or "\", fail parsing.
			if nextChar != _DQUOTE && nextChar != '\\' {
				return BareItem{}, "", fmt.Errorf("%w: invalid escape sequence", ErrInvalidString)
			}

			// 4. Append next_char to output_string.
			outputString.WriteString(inputString[outputStringEnd : curr-2])
			outputString.WriteByte(nextChar)
			outputStringEnd = curr
			continue
		}

		// 3. Else, if char is DQUOTE, return output_string.
		if char == _DQUOTE {
			if outputStringEnd == 1 {
				return BareItemString(inputString[1 : curr-1]), inputString[curr:], nil
			}
			return BareItemString(outputString.String()), inputString[curr:], nil
		}

		// 4. Else, if char is in the range %x00-1f or %x7f-ff (i.e., it is not in VCHAR or SP), fail parsing.
		if char <= 0x1f || char >= 0x7f {
			return BareItem{}, "", fmt.Errorf("%w: invalid character", ErrInvalidString)
		}

		// 5. Else, append char to output_string.
		// -> Optimized away
	}

	// 5. Reached the end of input_string without finding a closing DQUOTE; fail parsing.
	return BareItem{}, "", fmt.Errorf("%w: missing closing quote", ErrInvalidString)
}

// serializeBareItemString serializes a string as specified in RFC 9651 section 4.1.6 "Serializing a String".
func serializeBareItemString(dst []byte, inputString string) ([]byte, error) {
	// From https://www.rfc-editor.org/info/rfc9651/#section-4.1.6
	//
	// 1. Convert input_string into a sequence of ASCII characters; if conversion fails, fail serialization.
	if !isASCII(inputString) {
		return nil, prefixError(ErrInvalidString, ErrNonAsciiInput)
	}

	var toEscape int

	// 2. If input_string contains characters in the range %x00-1f or %x7f-ff (i.e., not in VCHAR or SP), fail serialization.
	for _, char := range []byte(inputString) {
		if char <= 0x1f || char >= 0x7f {
			return nil, fmt.Errorf("%w: invalid character", ErrInvalidString)
		}

		if char == '\\' || char == _DQUOTE {
			toEscape++
		}
	}

	dst = slices.Grow(dst, len(inputString)+2+toEscape)

	// 3. Let output be the string DQUOTE.
	output := append(dst, '"')

	// 4. For each character char in input_string:
	if toEscape == 0 {
		// 1. If char is "\" or DQUOTE:
		// -> We know that we do not need to escape anything

		// 2. Append char to output.
		// -> Since we do not need to escape anything we can just append the whole string at once
		output = append(output, inputString...)
	} else {
		for _, char := range []byte(inputString) {
			// 1. If char is "\" or DQUOTE:
			if char == '\\' || char == _DQUOTE {
				output = append(output, '\\')
			}

			// 2. Append char to output.
			output = append(output, char)
		}
	}

	// 5. Append DQUOTE to output.
	output = append(output, '"')

	// 6. Return output.
	return output, nil
}

// parseBareItemToken parses a token as specified in RFC 9651 section 4.2.6 "Parsing a Token".
func parseBareItemToken(inputString string) (v BareItem, rest string, err error) {
	// From https://www.rfc-editor.org/info/rfc9651/#section-4.2.6
	//
	// 1. If the first character of input_string is not ALPHA or "*", fail parsing
	if inputString == "" || (!isRFC5234ALPHA(inputString[0]) && inputString[0] != '*') {
		return BareItem{}, "", fmt.Errorf("%w: invalid or missing prefix character", ErrInvalidToken)
	}

	// 2. Let output_string be an empty string.
	// -> Replaced with index to avoid allocations
	var curr int

	// 3. While input_string is not empty:
	for curr < len(inputString) {
		// 1. If the first character of input_string is not in tchar, ":", or "/", return output_string.
		if !isRFC9110tchar(inputString[curr]) && inputString[curr] != ':' && inputString[curr] != '/' {
			return BareItemToken(inputString[:curr]), inputString[curr:], nil
		}

		// 2. Let char be the result of consuming the first character of input_string.
		// -> char optimized way
		curr++

		// 3. Append char to output_string.
		// -> Optimized away
	}

	// 4. Return output_string
	return BareItemToken(inputString[:curr]), inputString[curr:], nil
}

// serializeBareItemToken serializes a token as specified in RFC 9651 section 4.1.7 "Serializing a Token".
func serializeBareItemToken(dst []byte, inputToken string) ([]byte, error) {
	// From https://www.rfc-editor.org/info/rfc9651/#name-serializing-a-token
	//
	// 1. Convert input_token into a sequence of ASCII characters; if conversion fails, fail serialization.
	if !isASCII(inputToken) {
		return nil, prefixError(ErrInvalidToken, ErrNonAsciiInput)
	}

	// 2. If the first character of input_token is not ALPHA or "*", or the remaining portion contains a character not
	// in tchar, ":", or "/", fail serialization.
	if inputToken == "" || (!isRFC5234ALPHA(inputToken[0]) && inputToken[0] != '*') {
		return nil, ErrInvalidToken
	}

	for _, char := range []byte(inputToken[1:]) {
		if !isRFC9110tchar(char) && char != ':' && char != '/' {
			return nil, ErrInvalidToken
		}
	}

	// 3. Let output be an empty string.
	output := dst

	// 4. Append input_token to output.
	output = append(output, inputToken...)

	// 5. Return output.
	return output, nil
}

// parseBareItemByteSequence parses a byte sequence as specified in RFC 9651 section 4.2.7 "Parsing a Byte Sequence".
func parseBareItemByteSequence(inputString string) (v BareItem, rest string, err error) {
	// From https://www.rfc-editor.org/info/rfc9651/#name-parsing-a-byte-sequence
	//
	// 1. If the first character of input_string is not ":", fail parsing.
	if inputString == "" || inputString[0] != ':' {
		return BareItem{}, "", fmt.Errorf("%w: invalid or missing prefix character", ErrInvalidByteSequence)
	}
	// 2. Discard the first character of input_string
	inputString = inputString[1:]

	// 3. If there is not a ":" character before the end of input_string, fail parsing.
	endIdx := strings.IndexByte(inputString, ':')
	if endIdx == -1 {
		return BareItem{}, "", fmt.Errorf("%w: missing ending colon", ErrInvalidByteSequence)
	}

	// 4. Let b64_content be the result of consuming content of input_string up to but not including the first instance of the character ":".
	b64Content, inputString := inputString[:endIdx], inputString[endIdx:]

	// 5. Consume the ":" character at the beginning of input_string.
	inputString = inputString[1:]

	// 6. If b64_content contains a character not included in ALPHA, DIGIT, "+", "/", and "=", fail parsing.
	for _, c := range []byte(b64Content) {
		if isRFC5234ALPHA(c) || isRFC5234DIGIT(c) || c == '+' || c == '/' || c == '=' {
			continue
		}
		return BareItem{}, "", fmt.Errorf("%w: invalid character in base64 content", ErrInvalidByteSequence)
	}

	// 7. Let binary_content be the result of base64-decoding [RFC4648] b64_content, synthesizing padding if necessary (note the requirements about recipient behavior below). If base64 decoding fails, parsing fails.
	binaryContent, err := base64.StdEncoding.DecodeString(b64Content)
	if err != nil {
		// Because some implementations of base64 do not allow rejection of encoded data that is not properly "=" padded
		// (see [RFC4648], Section 3.2), parsers SHOULD NOT fail when "=" padding is not present, unless they cannot be
		// configured to do so.
		binaryContent, err = base64.RawStdEncoding.DecodeString(b64Content)
	}
	if err != nil {
		return BareItem{}, "", prefixError(ErrInvalidByteSequence, err)
	}

	// 8. Return binary_content.
	return BareItemByteSequence(binaryContent), inputString, nil
}

// serializeBareItemByteSequence serializes a byte sequence as specified in RFC 9651 section 4.1.8 "Serializing a Byte Sequence".
func serializeBareItemByteSequence(dst []byte, inputBytes []byte) ([]byte, error) {
	// https://www.rfc-editor.org/info/rfc9651/#name-serializing-a-byte-sequence
	//
	// 1. If input_bytes is not a sequence of bytes, fail serialization.
	// Note: Not applicable

	dst = slices.Grow(dst, 2+base64.StdEncoding.EncodedLen(len(inputBytes)))

	// 2. Let output be an empty string.
	output := dst

	// 3. Append ":" to output.
	output = append(output, ':')

	// 4. Append the result of base64-encoding input_bytes as per [RFC4648], Section 4, taking account of the requirements below.
	output = base64.StdEncoding.AppendEncode(output, inputBytes)

	// 5. Append ":" to output.
	output = append(output, ':')

	// 6. Return output.
	return output, nil
}

// parseBareItemBoolean parses a boolean as specified in RFC 9651 section 4.2.8 "Parsing a Boolean".
func parseBareItemBoolean(inputString string) (v BareItem, rest string, err error) {
	// https://www.rfc-editor.org/info/rfc9651/#section-4.2.8
	//
	// 1. If the first character of input_string is not "?", fail parsing
	if inputString == "" || inputString[0] != '?' {
		return BareItem{}, "", fmt.Errorf("%w: invalid or missing prefix character", ErrInvalidBoolean)
	}

	// 2. Discard the first character of input_string.
	inputString = inputString[1:]

	// 3. If the first character of input_string matches "1", discard the first character, and return true.
	if inputString != "" && inputString[0] == '1' {
		inputString = inputString[1:]
		return BareItemBoolean(true), inputString, nil
	}

	// 4. If the first character of input_string matches "0", discard the first character, and return false.
	if inputString != "" && inputString[0] == '0' {
		inputString = inputString[1:]
		return BareItemBoolean(false), inputString, nil
	}

	// 5. No value has matched; fail parsing.
	return BareItem{}, "", ErrInvalidBoolean
}

// serializeBareItemBoolean serializes a boolean as specified in RFC 9651 section 4.1.9 "Serializing a Boolean".
func serializeBareItemBoolean(dst []byte, inputBoolean bool) ([]byte, error) {
	// https://www.rfc-editor.org/info/rfc9651/#name-serializing-a-boolean
	//
	// 1. If input_boolean is not a boolean, fail serialization.
	// Note: Not applicable

	// 2. Let output be an empty string.
	output := dst

	if inputBoolean {
		// 3. Append "?" to output.
		// 4. If input_boolean is true, append "1" to output.
		output = append(output, '?', '1')
	} else {
		// 3. Append "?" to output.
		// 5. If input_boolean is false, append "0" to output.
		output = append(output, '?', '0')
	}

	// 6. Return output.
	return output, nil
}

// parseBareItemDate parses a date as specified in RFC 9651 section 4.2.9 "Parsing a Date".
func parseBareItemDate(inputString string) (v BareItem, rest string, err error) {
	// From https://www.rfc-editor.org/info/rfc9651/#section-4.2.9
	//
	// 1. If the first character of input_string is not "@", fail parsing.
	if inputString == "" || inputString[0] != '@' {
		return BareItem{}, "", fmt.Errorf("%w: invalid or missing prefix character", ErrInvalidDate)
	}

	// 2. Discard the first character of input_string.
	inputString = inputString[1:]

	// 3. Let output_date be the result of running Parsing an Integer or Decimal (Section 4.2.4) with input_string.
	v, inputString, err = parseBareItemIntegerOrDecimal(inputString)
	if err != nil {
		return BareItem{}, "", ErrInvalidDate
	}

	// 4. If output_date is a Decimal, fail parsing.
	if v.Type() == BareItemTypeDecimal {
		return BareItem{}, "", ErrInvalidDate
	}

	// 5. Return output_date.
	return BareItemDate(v.Integer()), inputString, nil
}

// serializeBareItemDate serializes a date as specified in RFC 9651 section 4.1.10 "Serializing a Date".
func serializeBareItemDate(dst []byte, inputDate int64) ([]byte, error) {
	// https://www.rfc-editor.org/info/rfc9651/#section-4.1.10
	//
	// 1. Let output be "@".
	output := append(dst, '@')

	// 2. Append to output the result of running Serializing an Integer with input_date (Section 4.1.4).
	output, err := serializeBareItemInteger(output, inputDate)

	// 3. Return output.
	return output, err
}

// parseBareItemDisplayString parses a display string as specified in RFC 9651 section 4.2.10 "Parsing a Display String".
func parseBareItemDisplayString(inputString string) (v BareItem, rest string, err error) {
	// From https://www.rfc-editor.org/info/rfc9651/#section-4.2.10
	//
	// 1. If the first two characters of input_string are not "%" followed by DQUOTE, fail parsing.
	if len(inputString) < 2 || inputString[0] != '%' || inputString[1] != _DQUOTE {
		return BareItem{}, "", fmt.Errorf("%w: invalid or missing prefix characters", ErrInvalidDisplayString)
	}

	// 2. Discard the first two characters of input_string.
	inputString = inputString[2:]

	// 3. Let byte_array be an empty byte array.
	// -> Replaced with index to avoid allocations
	var curr int

	var byteArray []byte
	byteArrayEnd := 0

	// 4. While input_string is not empty:
	for curr < len(inputString) {
		// 1. Let char be the result of consuming the first character of input_string.
		char := inputString[curr]
		curr++

		// 2. If char is in the range %x00-1f or %x7f-ff (i.e., it is not in VCHAR or SP), fail parsing.
		if char <= 0x1f || char >= 0x7f {
			return BareItem{}, "", fmt.Errorf("%w: invalid character", ErrInvalidDisplayString)
		}

		// 3. If char is "%":
		if char == '%' {
			// 1. Let octet_hex be the result of consuming two characters from input_string. If there are not two characters, fail parsing.
			if len(inputString[curr:]) < 2 {
				return BareItem{}, "", fmt.Errorf("%w: missing characters after %%", ErrInvalidDisplayString)
			}
			octetHex := inputString[curr : curr+2]
			curr += 2

			// 2. if octet_hex contains characters outside the range %x30-39 or %x61-66 (i.e., it is not in 0-9 or lowercase a-f), fail parsing.
			// nolint:staticcheck
			if c := octetHex[0]; !((c >= 0x30 && c <= 0x39) || (c >= 0x61 && c <= 0x66)) {
				return BareItem{}, "", fmt.Errorf("%w: invalid hex character", ErrInvalidDisplayString)
			}
			// nolint:staticcheck
			if c := octetHex[1]; !((c >= 0x30 && c <= 0x39) || (c >= 0x61 && c <= 0x66)) {
				return BareItem{}, "", fmt.Errorf("%w: invalid hex character", ErrInvalidDisplayString)
			}

			// 3. Let octet be the result of hex decoding octet_hex (Section 8 of [RFC4648]).
			octet := decodeHex(octetHex[0])*16 + decodeHex(octetHex[1])

			// 4. Append octet to byte_array.
			byteArray = append(byteArray, inputString[byteArrayEnd:curr-3]...)
			byteArray = append(byteArray, octet)
			byteArrayEnd = curr
			continue
		}

		// 4. If char is DQUOTE:
		if char == _DQUOTE {
			// 1. Let unicode_sequence be the result of decoding byte_array as a UTF-8 string (Section 3 of [UTF8]). Fail parsing if decoding fails.
			var unicodeSequence string
			if byteArray == nil {
				unicodeSequence = inputString[:curr-1]
			} else {
				unicodeSequence = string(byteArray)
			}

			if !utf8.ValidString(unicodeSequence) {
				return BareItem{}, "", fmt.Errorf("%w: invalid UTF-8 string", ErrInvalidDisplayString)
			}

			// 2. Return unicode_sequence:
			return BareItemDisplayString(unicodeSequence), inputString[curr:], nil
		}

		// 5. Otherwise, if char is not %" or DQUOTE:
		{
			// -> Optimization
			if byteArray == nil {
				continue
			}

			// 1. Let byte be the result of applying ASCII encoding to char.
			byte_ := char // already ASCII?

			// 2. Append byte to byte_array
			byteArray = append(byteArray, byte_)
		}
	}

	// 5. Reached the end of input_string without finding a closing DQUOTE; fail parsing.
	return BareItem{}, "", fmt.Errorf("%w: missing closing quote", ErrInvalidDisplayString)
}

// serializeBareItemDisplayString serializes a display string as specified in RFC 9651 section 4.1.11 "Serializing a Display String".
func serializeBareItemDisplayString(dst []byte, inputSequence string) ([]byte, error) {
	// https://www.rfc-editor.org/info/rfc9651/#section-4.1.11
	//
	// 1. If input_sequence is not a sequence of Unicode code points, fail serialization.
	var nonASCII int

	for i := 0; i < len(inputSequence); {
		r, sz := utf8.DecodeRuneInString(inputSequence[i:])

		// See point 1 above.
		if r == utf8.RuneError {
			return nil, fmt.Errorf("%w: invalid UTF-8", ErrInvalidDisplayString)
		}

		// See point 2 below.
		if unicode.Is(unicode.Cs, r) {
			return nil, fmt.Errorf("%w: invalid unicode", ErrInvalidDisplayString)
		}

		i += sz

		if sz > 1 || (byte(r) == '%' || byte(r) == _DQUOTE || byte(r) <= 0x1f || byte(r) >= 0x7f) {
			nonASCII++
		}
	}

	// 2. Let byte_array be the result of applying UTF-8 encoding (Section 3 of [UTF8]) to input_sequence. If encoding
	//    fails, fail serialization.
	byteArray := inputSequence

	dst = slices.Grow(dst, 3+len(byteArray)+2*nonASCII)

	// 3. Let encoded_string be a string containing "%" followed by DQUOTE.
	encodedString := append(dst, '%', _DQUOTE)

	// 4. For each byte in byte_array:
	if nonASCII == 0 {
		// 1. If byte is %x25 ("%"), %x22 (DQUOTE), or in the ranges %x00-1f or %x7f-ff:
		// -> We know that no byte needs encoding, so we can skip this.

		// 2. Otherwise, decode byte as an ASCII character and append the result to encoded_string.
		// -> Since nothing needs encoding and everything is ASCII we can just append the characters directly.
		encodedString = append(encodedString, byteArray...)
	} else {
		for _, byte_ := range []byte(byteArray) {
			// 1. If byte is %x25 ("%"), %x22 (DQUOTE), or in the ranges %x00-1f or %x7f-ff:
			if byte_ == '%' || byte_ == _DQUOTE || byte_ <= 0x1f || byte_ >= 0x7f {
				// 1. Append "%" to encoded_string.
				encodedString = append(encodedString, '%')

				// 2. Let encoded_byte be the result of applying base16 encoding (Section 8 of [RFC4648]) to byte, with
				// any alphabetic characters converted to lowercase.
				encodedByte1, encodedByte2 := encodeHex(byte_)

				// 3. Append encoded_byte to encoded_string.
				encodedString = append(encodedString, encodedByte1, encodedByte2)
				continue
			}

			// 2. Otherwise, decode byte as an ASCII character and append the result to encoded_string.
			encodedString = append(encodedString, byte_)
		}
	}

	// 5. Append DQUOTE to encoded_string.
	encodedString = append(encodedString, _DQUOTE)

	// 6. Return encoded_string.
	return encodedString, nil
}

var _ encoding.TextAppender = (*BareItem)(nil)

// AppendText implements the [encoding.TextAppender] interface.
//
// It panics if b's type is not a valid value.
func (b *BareItem) AppendText(text []byte) ([]byte, error) {
	return serializeBareItem(text, *b)
}

// Type returns the type of the bare item.
func (b *BareItem) Type() BareItemType {
	return b.typ
}

// String returns the type name, which is the name of the constant minus the type prefix.
func (t BareItemType) String() string {
	switch t {
	case BareItemTypeBoolean:
		return "Boolean"
	case BareItemTypeByteSequence:
		return "ByteSequence"
	case BareItemTypeDate:
		return "Date"
	case BareItemTypeDecimal:
		return "Decimal"
	case BareItemTypeDisplayString:
		return "DisplayString"
	case BareItemTypeInteger:
		return "Integer"
	case BareItemTypeString:
		return "String"
	case BareItemTypeToken:
		return "Token"
	default:
		panic(fmt.Sprintf("unknown BareItemType %d", uint8(t)))
	}
}

// Dictionary represents an ordered map of key-values pairs as specified in RFC 9651 section 3.2.
type Dictionary struct {
	ordered.Map[string, ItemOrInnerList]
}

// parseDictionary parses a dictionary as specified in RFC 9651 section 4.2.2 "Parsing a Dictionary".
func parseDictionary(inputString string) (v Dictionary, rest string, err error) {
	// From https://www.rfc-editor.org/info/rfc9651/#name-parsing-a-dictionary
	//
	// 1. Let dictionary be an empty, ordered map.
	var members ordered.Map[string, ItemOrInnerList]

	// 2. While input_string is not empty:
	for inputString != "" {
		// 1. Let this_key be the result of running Parsing a Key (Section 4.2.3.3) with input_string.
		var thisKey string
		thisKey, inputString, err = parseKey(inputString)
		if err != nil {
			return Dictionary{}, "", ErrInvalidKey
		}

		var member ItemOrInnerList

		// 2. If the first character of input_string is "=":
		if inputString != "" && inputString[0] == '=' {
			// 1. Consume the first character of input_string.
			inputString = inputString[1:]

			// 2. Let member be the result of running Parsing an Item or Inner List (Section 4.2.1.1) with input_string.
			member, inputString, err = parseItemOrInnerList(inputString)
			if err != nil {
				return Dictionary{}, "", err
			}
		} else {
			// 3. Otherwise:

			{
				// 1. Let value be Boolean true.
				value := ItemOrInnerListFrom(Item{
					BareItem: BareItemBoolean(true),
				})

				// 2. Let parameters be the result of running Parsing Parameters (Section 4.2.3.2) with input_string.
				value.item.Parameters, inputString, err = parseParameters(inputString)
				if err != nil {
					return Dictionary{}, "", err
				}

				// 3. Let member be the tuple (value, parameters).
				member = value
			}
		}

		// 4. If dictionary already contains a key this_key (comparing character for character), overwrite its value with member.
		// 5. Otherwise, append key this_key with value member to dictionary.
		members.Set(thisKey, member)

		// 6. Discard any leading OWS characters from input_string.
		inputString = trimLeadingOWS(inputString)

		// 7. If input_string is empty, return dictionary.
		if inputString == "" {
			return Dictionary{members}, inputString, nil
		}

		// 8. Consume the first character of input_string; if it is not ",", fail parsing.
		if inputString[0] != ',' {
			return Dictionary{}, "", fmt.Errorf("%w: invalid character after dictionary member", ErrInvalidDictionary)
		}
		inputString = inputString[1:]

		// 9. Discard any leading OWS characters from input_string.
		inputString = trimLeadingOWS(inputString)

		// 10. If input_string is empty, there is a trailing comma; fail parsing.
		if inputString == "" {
			return Dictionary{}, "", fmt.Errorf("%w: trailing comma", ErrInvalidDictionary)
		}
	}

	// 3. No structured data has been found; return dictionary (which is empty).
	return Dictionary{members}, inputString, nil
}

// serializeDictionary serializes a list as specified in RFC 9651 section 4.1.2 "Serializing a Dictionary".
func serializeDictionary(dst []byte, inputDictionary Dictionary) ([]byte, error) {
	var err error

	// From https://www.rfc-editor.org/info/rfc9651/#section-4.1.2
	//
	// 1. Let output be an empty string..
	output := dst

	var idx int

	// 2. For each member_key with a value of (member_value, parameters) in input_dictionary:
	for memberKey, memberValue := range inputDictionary.All() {
		// 1. Append the result of running Serializing a Key (Section 4.1.1.3) with member's member_key to output.
		output, err = serializeKey(output, memberKey)
		if err != nil {
			return nil, prefixError(ErrInvalidDictionary, err)
		}

		switch {
		// 2. If member_value is Boolean true:
		case memberValue.Type() == ItemOrInnerListTypeItem &&
			memberValue.item.Type() == BareItemTypeBoolean &&
			memberValue.item.Boolean():
			// 1. Append the result of running Serializing Parameters (Section 4.1.1.2) with parameters to output.
			output, err = serializeParameters(output, memberValue.item.Parameters)
		// 3. Otherwise:
		default:
			// 1. Append "=" to output.
			output = append(output, '=')

			switch memberValue.Type() {
			// 2. If member_value is an array, append the result of running Serializing an Inner List (Section 4.1.1.1) with (member_value, parameters) to output.
			case ItemOrInnerListTypeInnerList:
				output, err = serializeInnerList(output, memberValue.innerList)
			// 3. Otherwise, append the result of running Serializing an Item (Section 4.1.3) with (member_value, parameters) to output.
			case ItemOrInnerListTypeItem:
				output, err = serializeItem(output, memberValue.item)
			}
		}

		if err != nil {
			return nil, prefixError(ErrInvalidDictionary, err)
		}

		// 4. If more members remain in input_dictionary:
		if idx < inputDictionary.Len()-1 {
			// 1. Append "," to output.
			// 2. Append a single SP to output.
			output = append(output, ',', _SP)
		}

		idx++
	}

	// 3. Return output.
	return output, nil
}

var _ encoding.TextAppender = (*Dictionary)(nil)

// AppendText implements the [encoding.TextAppender] interface.
func (d *Dictionary) AppendText(text []byte) ([]byte, error) {
	return serializeDictionary(text, *d)
}

// InnerList represents an inner list as specified in RFC 9651 section 3.1.1.
type InnerList struct {
	// Members contains the members of the list.
	Members []Item

	// Parameters optionally contains the parameters specified for the value.
	Parameters Parameters
}

// parseInnerList parses an inner list as specified in RFC 9651 section 4.2.1.2 "Parsing an Inner List".
func parseInnerList(inputString string) (v InnerList, rest string, err error) {
	// From https://www.rfc-editor.org/info/rfc9651/#parse-innerlist
	//
	// 1. Consume the first character of input_string; if it is not "(", fail parsing.
	if inputString == "" || inputString[0] != '(' {
		// Note: Unreachable as parseInnerList is only called when we already checked the first character.
		return InnerList{}, "", fmt.Errorf("%w: invalid or missing prefix character", ErrInvalidInnerList)
	}
	inputString = inputString[1:]

	// 1. Let inner_list be an empty array.
	var innerList InnerList

	// 2. While input_string is not empty:
	for inputString != "" {
		// 1. Discard any leading SP characters from input_string.
		inputString = trimLeadingSP(inputString)

		// 2. If the first character of input_string is ")":
		if inputString != "" && inputString[0] == ')' {
			// 1. Consume the first character of input_string.
			inputString = inputString[1:]

			// 2. Let parameters be the result of running Parsing Parameters (Section 4.2.3.2) with input_string.
			innerList.Parameters, inputString, err = parseParameters(inputString)
			if err != nil {
				return InnerList{}, "", prefixError(ErrInvalidInnerList, err)
			}

			// 3. Return the tuple (inner_list, parameters).
			return innerList, inputString, nil
		}

		// 3. Let item be the result of running Parsing an Item (Section 4.2.3) with input_string.
		var item Item
		item, inputString, err = parseItem(inputString)
		if err != nil {
			return InnerList{}, "", err
		}

		// 4. Append item to inner_list.
		// Removes some allocations for lists with more than 2 items
		if innerList.Members == nil {
			innerList.Members = make([]Item, 0, 4)
		}
		innerList.Members = append(innerList.Members, item)

		// 5. If the first character of input_string is not SP or ")", fail parsing.
		if inputString == "" || (inputString[0] != _SP && inputString[0] != ')') {
			return InnerList{}, "", fmt.Errorf("%w: unexpected end of inner list", ErrInvalidInnerList)
		}
	}

	// 4. The end of the Inner List was not found; fail parsing.
	return InnerList{}, "", fmt.Errorf("%w: unexpected end of inner list", ErrInvalidInnerList)
}

// serializeInnerList serializes an inner list as specified in RFC 9651 section 4.1.1.2 "Serializing an Inner List".
func serializeInnerList(dst []byte, il InnerList) ([]byte, error) {
	var err error

	// From https://www.rfc-editor.org/info/rfc9651/#name-serializing-an-inner-list
	//
	// 1. Let output be the string "(".
	output := append(dst, '(')

	// 2. For each (member_value, parameters) of inner_list:
	for i, member := range il.Members {
		// 1. Append the result of running Serializing an Item (Section 4.1.3) with (member_value, parameters) to output.
		output, err = serializeItem(output, member)
		if err != nil {
			return nil, prefixError(ErrInvalidInnerList, err)
		}

		// 2. If more values remain in inner_list, append a single SP to output.
		if i < len(il.Members)-1 {
			output = append(output, ' ')
		}
	}

	// 3. Append ")" to output.
	output = append(output, ')')

	// 4. Append the result of running Serializing Parameters (Section 4.1.1.2) with list_parameters to output.
	output, err = serializeParameters(output, il.Parameters)
	if err != nil {
		return nil, prefixError(ErrInvalidInnerList, err)
	}

	// 5. Return output.
	return output, nil
}

var _ encoding.TextAppender = (*InnerList)(nil)

// AppendText implements the [encoding.TextAppender] interface.
func (il *InnerList) AppendText(text []byte) ([]byte, error) {
	return serializeInnerList(text, *il)
}

// Item represents a single item with optional parameters as specified in RFC 9651 section 3.3.
type Item struct {
	// BareItem contains the bare item without parameters.
	BareItem

	// Parameters optionally contains the parameters specified for the value.
	Parameters Parameters
}

// parseItem parses a single item as specified in RFC 9651 section 4.3.2 "Parsing an Item".
func parseItem(inputString string) (v Item, rest string, err error) {
	// From https://www.rfc-editor.org/info/rfc9651/#section-4.2.3
	//
	// 1. Let bare_item be the result of running Parsing a Bare Item (Section 4.2.3.1) with input_string.
	item, inputString, err := parseBareItem(inputString)
	if err != nil {
		return Item{}, "", prefixError(ErrInvalidItem, err)
	}

	// 2. Let parameters be the result of running Parsing Parameters (Section 4.2.3.2) with input_string.
	params, inputString, err := parseParameters(inputString)
	if err != nil {
		return Item{}, "", prefixError(ErrInvalidItem, err)
	}

	// 3. Return the tuple (bare_item, parameters).
	return Item{BareItem: item, Parameters: params}, inputString, nil
}

// serializeItem serializes an item as specified in RFC 9651 section 4.1.3 "Serializing an item".
func serializeItem(dst []byte, i Item) ([]byte, error) {
	// From https://www.rfc-editor.org/info/rfc9651/#section-4.1.3
	//
	// Let output be an empty string.
	output := dst

	var err error

	// 2. Append the result of running Serializing a Bare Item (Section 4.1.3.1) with bare_item to output.
	output, err = serializeBareItem(output, i.BareItem)
	if err != nil {
		return nil, prefixError(ErrInvalidItem, err)
	}

	// 3. Append the result of running Serializing Parameters (Section 4.1.1.2) with item_parameters to output.
	output, err = serializeParameters(output, i.Parameters)
	if err != nil {
		return nil, prefixError(ErrInvalidItem, err)
	}

	// 4. Return output.
	return output, nil
}

var _ encoding.TextAppender = (*Item)(nil)

// AppendText implements the [encoding.TextAppender] interface.
func (i *Item) AppendText(text []byte) ([]byte, error) {
	return serializeItem(text, *i)
}

// ItemOrInnerList contains either an [Item] or [InnerList] as part of a [Dictionary] or [List].
//
// It acts as a tagged union, with [ItemOrInnerList.Type] specifying the type and [ItemOrInnerList.InnerList] or
// [ItemOrInnerList.Item] returning the value.
type ItemOrInnerList struct {
	typ       ItemOrInnerListType
	innerList InnerList
	item      Item
}

// ItemOrInnerListType is an enum of types that a ItemOrInnerList can contain.
type ItemOrInnerListType uint8

const (
	// ItemOrInnerListTypeInnerList denotes an ItemOrInnerList containing an InnerList.
	ItemOrInnerListTypeInnerList ItemOrInnerListType = iota

	// ItemOrInnerListTypeItem denotes an ItemOrInnerList containing an Item.
	ItemOrInnerListTypeItem
)

// ItemOrInnerListFrom wraps t in the tagged union ItemOrInnerList.
func ItemOrInnerListFrom[T InnerList | Item](t T) ItemOrInnerList {
	switch v := any(t).(type) {
	case InnerList:
		return ItemOrInnerList{typ: ItemOrInnerListTypeInnerList, innerList: v}
	case Item:
		return ItemOrInnerList{typ: ItemOrInnerListTypeItem, item: v}
	default:
		panic("unreachable")
	}
}

// parseList parses an item or inner list as specified in RFC 9651 section 4.2.1.1 "Parsing an Item or Inner List".
func parseItemOrInnerList(inputString string) (v ItemOrInnerList, rest string, err error) {
	// From https://www.rfc-editor.org/info/rfc9651/#parse-item-or-list
	//
	// 1. If the first character of input_string is "(", return the result of running Parsing an Inner List (Section 4.2.1.2) with input_string.
	if inputString != "" && inputString[0] == '(' {
		innerList, inputString, err := parseInnerList(inputString)
		if err != nil {
			return ItemOrInnerList{}, "", err
		}
		return ItemOrInnerList{typ: ItemOrInnerListTypeInnerList, innerList: innerList}, inputString, nil
	}

	// 2. Return the result of running Parsing an Item (Section 4.2.3) with input_string.
	item, inputString, err := parseItem(inputString)
	if err != nil {
		return ItemOrInnerList{}, "", err
	}
	return ItemOrInnerList{typ: ItemOrInnerListTypeItem, item: item}, inputString, nil
}

// InnerList returns the underlying InnerList.
//
// It panics if t is not of type [ItemOrInnerListTypeInnerList].
func (t ItemOrInnerList) InnerList() InnerList {
	if t.typ != ItemOrInnerListTypeInnerList {
		panic("ItemOrInnerList is not an InnerList")
	}

	return t.innerList
}

// Item returns the underlying Item.
//
// It panics if t is not of type [ItemOrInnerListTypeItem].
func (t ItemOrInnerList) Item() Item {
	if t.typ != ItemOrInnerListTypeItem {
		panic("ItemOrInnerList is not an Item")
	}

	return t.item
}

// Type returns the type of the underlying value.
func (t ItemOrInnerList) Type() ItemOrInnerListType {
	return t.typ
}

// String returns the type name, which is the name of the constant minus the type prefix.
func (t ItemOrInnerListType) String() string {
	switch t {
	case ItemOrInnerListTypeInnerList:
		return "InnerList"
	case ItemOrInnerListTypeItem:
		return "Item"
	default:
		panic(fmt.Sprintf("unknown ItemOrInnerListType %d", uint8(t)))
	}
}

// List represents a list of items with optional parameters as specified in RFC 9651 section 3.1.
type List struct {
	// Members is a slice of the members of the list.
	Members []ItemOrInnerList
}

// parseList parses a list as specified in RFC 9651 section 4.2.1 "Parsing a List".
func parseList(inputString string) (v List, rest string, err error) {
	// From https://www.rfc-editor.org/info/rfc9651/#name-parsing-a-list
	//
	// 1. Let members be an empty array.
	var members []ItemOrInnerList

	// 2. While input_string is not empty:
	for inputString != "" {
		// 1. Append the result of running Parsing an Item or Inner List (Section 4.2.1.1) with input_string to members.
		var member ItemOrInnerList
		member, inputString, err = parseItemOrInnerList(inputString)
		if err != nil {
			return List{}, "", prefixError(ErrInvalidList, err)
		}
		// Removes some allocations for lists with more than 2 items
		if members == nil {
			members = make([]ItemOrInnerList, 0, 4)
		}
		members = append(members, member)

		// 2. Discard any leading OWS characters from input_string.
		inputString = trimLeadingOWS(inputString)

		// 3. If input_string is empty, return members.
		if inputString == "" {
			return List{Members: members}, inputString, nil
		}

		// 4. Consume the first character of input_string; if it is not ",", fail parsing.
		if inputString[0] != ',' {
			return List{}, "", fmt.Errorf("%w: invalid character after list member", ErrInvalidList)
		}
		inputString = inputString[1:]

		// 5. Discard any leading OWS characters from input_string.
		inputString = trimLeadingOWS(inputString)

		// 6. If input_string is empty, there is a trailing comma; fail parsing.
		if inputString == "" {
			return List{}, "", fmt.Errorf("%w: trailing comma", ErrInvalidList)
		}
	}

	// 3. No structured data has been found; return members (which is empty).
	return List{}, inputString, nil
}

// serializeList serializes a list as specified in RFC 9651 section 4.1.1 "Serializing a List".
func serializeList(dst []byte, inputList List) ([]byte, error) {
	var err error

	// From https://www.rfc-editor.org/info/rfc9651/#name-serializing-a-list
	//
	// 1. Let output be an empty string.
	output := dst

	// 2. For each (member_value, parameters) of input_list:
	for i, member := range inputList.Members {
		switch member.Type() {
		case ItemOrInnerListTypeInnerList:
			// 1. If member_value is an array, append the result of running Serializing an Inner List (Section 4.1.1.1) with (member_value, parameters) to output.
			output, err = serializeInnerList(output, member.innerList)
		case ItemOrInnerListTypeItem:
			// 2. Otherwise, append the result of running Serializing an Item (Section 4.1.3) with (member_value, parameters) to output.
			output, err = serializeItem(output, member.item)
		}

		if err != nil {
			return nil, prefixError(ErrInvalidList, err)
		}

		// 3. If more member_values remain in input_list:
		if i < len(inputList.Members)-1 {
			// 1. Append "," to output.
			// 2. Append a single SP to output.
			output = append(output, ',', _SP)
		}
	}

	// 3. Return output.
	return output, nil
}

var _ encoding.TextAppender = (*List)(nil)

// AppendText implements the [encoding.TextAppender] interface.
func (l *List) AppendText(text []byte) ([]byte, error) {
	return serializeList(text, *l)
}

// Parameters is an ordered map of parameters that can be added to a [InnerList], [Item] or [List].
type Parameters struct {
	ordered.Map[string, BareItem]
}

// parseParameters parses parameters as specified in RFC 9651 section 4.2.3.2 "Parsing Parameters".
func parseParameters(inputString string) (v Parameters, rest string, err error) {
	// From https://www.rfc-editor.org/info/rfc9651/#name-parsing-parameters
	//
	// 1. Let parameters be an empty, ordered map.
	var m ordered.Map[string, BareItem]

	// 2. While input_string is not empty:
	for inputString != "" {
		// 1. If the first character of input_string is not ";", exit the loop.
		if inputString[0] != ';' {
			break
		}

		// 2. Consume the ";" character from the beginning of input_string.
		inputString = inputString[1:]

		// 3. Discard any leading SP characters from input_string.
		inputString = trimLeadingSP(inputString)

		// 4. Let param_key be the result of running Parsing a Key (Section 4.2.3.3) with input_string.
		var paramKey string
		paramKey, inputString, err = parseKey(inputString)
		if err != nil {
			return Parameters{}, inputString, prefixError(ErrInvalidParameters, err)
		}

		// 5. Let param_value be Boolean true.
		paramValue := BareItemBoolean(true)

		// 6. If the first character of input_string is "=":
		if inputString != "" && inputString[0] == '=' {
			// 1. Consume the "=" character at the beginning of input_string.
			inputString = inputString[1:]

			// 2. Let param_value be the result of running Parsing a Bare Item (Section 4.2.3.1) with input_string.
			paramValue, inputString, err = parseBareItem(inputString)
			if err != nil {
				return Parameters{}, inputString, prefixError(ErrInvalidParameters, err)
			}
		}

		// 7. If parameters already contains a key param_key (comparing character for character), overwrite its value with param_value.
		// 8. Otherwise, append key param_key with value param_value to parameters.
		m.Set(paramKey, paramValue)
	}

	// 3. Return parameters.
	return Parameters{m}, inputString, nil
}

// serializeParameters serializes parameters as specified in RFC 9651 section 4.1.1.2 "Serializing Parameters".
func serializeParameters(dst []byte, p Parameters) ([]byte, error) {
	// From https://www.rfc-editor.org/info/rfc9651/#name-serializing-parameters
	//
	// 1. Let output be an empty string.
	output := dst

	// 2. For each param_key with a value of param_value in input_parameters:
	for paramKey, paramValue := range p.All() {
		// 1. Append ";" to output.
		output = append(output, ';')

		// 2. Append the result of running Serializing a Key (Section 4.1.1.3) with param_key to output.
		var err error
		output, err = serializeKey(output, paramKey)
		if err != nil {
			return nil, prefixError(ErrInvalidParameters, err)
		}

		// 4. If param_value is not Boolean true:
		if paramValue.Type() != BareItemTypeBoolean || !paramValue.Boolean() {
			// 1. Append "=" to output.
			output = append(output, '=')

			// 2. Append the result of running Serializing a bare Item (Section 4.1.3.1) with param_value to output.
			output, err = serializeBareItem(output, paramValue)
			if err != nil {
				return nil, prefixError(ErrInvalidParameters, err)
			}
		}
	}

	// 3. Return output.
	return output, nil
}

var _ encoding.TextAppender = (*Parameters)(nil)

// AppendText implements the [encoding.TextAppender] interface.
func (p *Parameters) AppendText(text []byte) ([]byte, error) {
	return serializeParameters(text, *p)
}
