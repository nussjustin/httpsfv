// Package httpsfv implements parsing of HTTP Structured Field Values as specified in RFC 9651.
package httpsfv

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
	"unsafe"

	"github.com/nussjustin/httpsfv/ordered"
)

var (
	// ErrInvalidBareItem is returned when parsing a bare item fails as specified in RFC 9651 section 4.2.3.1 "Parsing a Bare Item".
	ErrInvalidBareItem = errors.New("invalid bare item")

	// ErrInvalidBoolean is returned when parsing a boolean fails as specified in RFC 9651 section 4.2.8 "Parsing a Boolean".
	ErrInvalidBoolean = errors.New("invalid boolean")

	// ErrInvalidByteSequence is returned when parsing a byte sequence fails as specified in RFC 9651 section 4.2.7 "Parsing a Byte Sequence".
	ErrInvalidByteSequence = errors.New("invalid byte sequence")

	// ErrInvalidDate is returned when parsing a date fails as specified in RFC 9651 section 4.2.9 "Parsing a Date".
	ErrInvalidDate = errors.New("invalid date")

	// ErrInvalidDictionary is returned when parsing a dictionary fails as specified in RFC 9651 section 4.2.2 "Parsing a Dictionary".
	ErrInvalidDictionary = errors.New("invalid dictionary")

	// ErrInvalidDisplayString is returned when parsing a display string fails as specified in RFC 9651 section 4.2.10 "Parsing a Display String".
	ErrInvalidDisplayString = errors.New("invalid display string")

	// ErrInvalidInnerList is returned when parsing an inner list fails as specified in RFC 9651 section 4.2.1.2 "Parsing an Inner List".
	ErrInvalidInnerList = errors.New("invalid inner list")

	// ErrInvalidIntegerOrDecimal is returned when parsing an integer or decimal fails as specified in RFC 9651 section 4.2.4 "Parsing an Integer or Decimal".
	ErrInvalidIntegerOrDecimal = errors.New("invalid integer or decimal")

	// ErrInvalidKey is returned when parsing a key fails as specified in RFC 9651 section 4.2.3.3 "Parsing a Key".
	ErrInvalidKey = errors.New("invalid key")

	// ErrInvalidList is returned when parsing a list fails as specified in RFC 9651 section 4.2.1 "Parsing a List".
	ErrInvalidList = errors.New("invalid list")

	// ErrInvalidString is returned when parsing a string fails as specified in RFC 9651 section 4.2.5 "Parsing a String".
	ErrInvalidString = errors.New("invalid string")

	// ErrInvalidToken is returned when parsing a token fails as specified in RFC 9651 section 4.2.6 "Parsing a Token".
	ErrInvalidToken = errors.New("invalid token")

	// ErrNonAsciiInput is returned when parsing a structured field containing non-ASCII characters as specified in RFC 9651 section 4.2 "Parsing Structured Fields".
	ErrNonAsciiInput = errors.New("non-ascii input")

	// ErrTrailingData is returned when parsing a structured field containing trailing data as specified in RFC 9651 section 4.2 "Parsing Structured Fields".
	ErrTrailingData = errors.New("trailing data")
)

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
	if !isAscii(inputString) {
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
// Otherwise, the result of parsing the strings as one, joined by comma and space, is returned.
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

func isAscii(s string) bool {
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
	var first byte
	if inputString != "" {
		first = inputString[0]
	}

	if !isLcalpha(first) && first != '*' {
		return "", "", ErrInvalidKey
	}

	// 2. Let output_string be an empty string.
	// TODO: Optimize
	var outputString string

	// 3. While input_string is not empty:
	for inputString != "" {
		// 1. If the first character of input_string is not one of lcalpha, DIGIT, "_", "-", ".", or "*", return output_string.
		switch {
		case isLcalpha(inputString[0]):
		case isRFC5234DIGIT(inputString[0]):
		case inputString[0] == '_':
		case inputString[0] == '-':
		case inputString[0] == '.':
		case inputString[0] == '*':
		default:
			return outputString, inputString, nil
		}

		// 2. Let char be the result of consuming the first character of input_string.
		var char byte
		char, inputString = inputString[0], inputString[1:]

		// 3. Append char to output_string.
		outputString += string(char)
	}

	// 4. Return output_string.
	return outputString, inputString, nil
}

// BareItem represents a simple item without parameters.
//
// It acts as a tagged union, with [BareItem.Type] returning the type of the item.
type BareItem struct {
	// Type is used as tag to denote the field in which the value is stored.
	Type BareItemType

	// Boolean contains the value if Type is BareItemTypeBoolean.
	Boolean bool

	// ByteSequence contains the value if Type is BareItemTypeByteSequence.
	ByteSequence []byte

	// Date contains the value if Type is BareItemTypeDate.
	Date int64

	// Decimal contains the value if Type is BareItemTypeDecimal.
	Decimal float64

	// DisplayString contains the value if Type is BareItemTypeDisplayString.
	DisplayString string

	// Integer contains the value if Type is BareItemTypeInteger.
	Integer int64

	// String contains the value if Type is BareItemTypeString.
	String string

	// Token contains the value if Type is BareItemTypeToken.
	Token string
}

// BareItemType is an enum of types for a BareItem.
type BareItemType uint8

const (
	// BareItemTypeBoolean denotes a boolean value as specified in RFC 9651 section 3.3.6.
	BareItemTypeBoolean BareItemType = iota

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
		return parseIntegerOrDecimal(inputString)
	case first == _DQUOTE:
		// 2. If the first character of input_string is a DQUOTE, return the result of running Parsing a String (Section 4.2.5) with input_string.
		return parseString(inputString)
	case isRFC5234ALPHA(first) || first == '*':
		// 3. If the first character of input_string is an ALPHA or "*", return the result of running Parsing a Token (Section 4.2.6) with input_string
		return parseToken(inputString)
	case first == ':':
		// 4. If the first character of input_string is ":", return the result of running Parsing a Byte Sequence (Section 4.2.7) with input_string.
		return parseByteSequence(inputString)
	case first == '?':
		// 5. If the first character of input_string is "?", return the result of running Parsing a Boolean (Section 4.2.8) with input_string.
		return parseBoolean(inputString)
	case first == '@':
		// 6. If the first character of input_string is "@", return the result of running Parsing a Date (Section 4.2.9) with input_string.
		return parseDate(inputString)
	case first == '%':
		// 7. If the first character of input_string is "%", return the result of running Parsing a Display String (Section 4.2.10) with input_string.
		return parseDisplayString(inputString)
	default:
		// 8. Otherwise, the item type is unrecognized; fail parsing
		return BareItem{}, "", ErrInvalidBareItem
	}
}

// parseIntegerOrDecimal parses an integer or decimal as specified in RFC 9651 section 4.3.4 "Parsing an Integer or Decimal".
func parseIntegerOrDecimal(inputString string) (v BareItem, rest string, err error) {
	// From https://www.rfc-editor.org/info/rfc9651/#name-parsing-an-integer-or-decim
	//
	// 1. Let type be "integer".
	type_ := BareItemTypeInteger

	// 2. Let sign be 1.
	sign := 1

	// 3. Let input_number be an empty string.
	// TODO: Optimize to avoid allocations
	var inputNumber string

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
	for inputString != "" {
		var char byte

		// 1. Let char be the result of consuming the first character of input_string.
		char, inputString = inputString[0], inputString[1:]

		// 2. If char is a DIGIT, append it to input_number.
		if isRFC5234DIGIT(char) {
			inputNumber += string(char)
			continue
		}

		// 3. Else, if type is "integer" and char is ".":
		if type_ == BareItemTypeInteger && char == '.' {
			// 1. If input_number contains more than 12 characters, fail parsing.
			if len(inputNumber) > 12 {
				return BareItem{}, "", fmt.Errorf("%w: integer part too long", ErrInvalidIntegerOrDecimal)
			}

			// 2. Otherwise, append char to input_number and set type to "decimal".
			inputNumber += string(char)
			type_ = BareItemTypeDecimal
			continue
		}

		// 4. Otherwise, prepend char to input_string, and exit the loop.
		inputString = string(char) + inputString
		break
	}

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
		return BareItem{Type: type_, Integer: outputNumberInt}, inputString, nil
	}

	return BareItem{Type: type_, Decimal: outputNumberFloat}, inputString, nil
}

// parseString parses a string as specified in RFC 9651 section 4.2.5 "Parsing a String".
func parseString(inputString string) (v BareItem, rest string, err error) {
	// From https://www.rfc-editor.org/info/rfc9651/#section-4.2.5
	//
	// 1. Let output_string be an empty string.
	// TODO: Optimize
	var outputString string

	// 2. If the first character of input_string is not DQUOTE, fail parsing.
	if inputString == "" || inputString[0] != _DQUOTE {
		// Note: Unreachable as parseString is only called when we already checked the first character.
		return BareItem{}, "", fmt.Errorf("%w: invalid or missing prefix character", ErrInvalidString)
	}

	// 3. Discard the first character of input_string.
	inputString = inputString[1:]

	// 4. While input_string is not empty:
	for inputString != "" {
		// 1. Let char be the result of consuming the first character of input_string.
		var char byte
		char, inputString = inputString[0], inputString[1:]

		// 2. If char is a backslash ("\"):
		if char == '\\' {
			// 1. If input_string is now empty, fail parsing.
			if inputString == "" {
				return BareItem{}, "", fmt.Errorf("%w: invalid escape sequence", ErrInvalidString)
			}

			// 2. Let next_char be the result of consuming the first character of input_string.
			var nextChar byte
			nextChar, inputString = inputString[0], inputString[1:]

			// 3. If next_char is not DQUOTE or "\", fail parsing.
			if nextChar != _DQUOTE && nextChar != '\\' {
				return BareItem{}, "", fmt.Errorf("%w: invalid escape sequence", ErrInvalidString)
			}

			// 4. Append next_char to output_string.
			outputString += string(nextChar)
			continue
		}

		// 3. Else, if char is DQUOTE, return output_string.
		if char == _DQUOTE {
			return BareItem{Type: BareItemTypeString, String: outputString}, inputString, nil
		}

		// 4. Else, if char is in the range %x00-1f or %x7f-ff (i.e., it is not in VCHAR or SP), fail parsing.
		if char <= 0x1f || char >= 0x7f {
			return BareItem{}, "", fmt.Errorf("%w: invalid character", ErrInvalidString)
		}

		// 5. Else, append char to output_string.
		outputString += string(char)
	}

	// 5. Reached the end of input_string without finding a closing DQUOTE; fail parsing.
	return BareItem{}, "", fmt.Errorf("%w: missing closing quote", ErrInvalidString)
}

// parseToken parses a token as specified in RFC 9651 section 4.2.6 "Parsing a Token".
func parseToken(inputString string) (v BareItem, rest string, err error) {
	// From https://www.rfc-editor.org/info/rfc9651/#section-4.2.6
	//
	// 1. If the first character of input_string is not ALPHA or "*", fail parsing
	if inputString == "" || (!isRFC5234ALPHA(inputString[0]) && inputString[0] != '*') {
		// Note: Unreachable as parseToken is only called when we already checked the first character.
		return BareItem{}, "", fmt.Errorf("%w: invalid or missing prefix character", ErrInvalidToken)
	}

	// 2. Let output_string be an empty string.
	// TODO: Optimize
	var outputString string

	// 3. While input_string is not empty:
	for inputString != "" {
		// 1. If the first character of input_string is not in tchar, ":", or "/", return output_string.
		if !isRFC9110tchar(inputString[0]) && inputString[0] != ':' && inputString[0] != '/' {
			return BareItem{Type: BareItemTypeToken, Token: outputString}, inputString, nil
		}

		// 2. Let char be the result of consuming the first character of input_string.
		var char byte
		char, inputString = inputString[0], inputString[1:]

		// 3. Append char to output_string.
		outputString += string(char)
	}

	// 4. Return output_string
	return BareItem{Type: BareItemTypeToken, Token: outputString}, inputString, nil
}

// parseByteSequence parses a byte sequence as specified in RFC 9651 section 4.2.7 "Parsing  Byte Sequence".
func parseByteSequence(inputString string) (v BareItem, rest string, err error) {
	// From https://www.rfc-editor.org/info/rfc9651/#name-parsing-a-byte-sequence
	//
	// 1. If the first character of input_string is not ":", fail parsing.
	if inputString == "" || inputString[0] != ':' {
		// Note: Unreachable as parseByteSequence is only called when we already checked the first character.
		return BareItem{}, "", fmt.Errorf("%w: invalid or missing prefix character", ErrInvalidByteSequence)
	}

	// 2. Discard the first character of input_string.
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
		return BareItem{}, "", fmt.Errorf("%w: %s", ErrInvalidByteSequence, err)
	}

	// 8. Return binary_content.
	return BareItem{Type: BareItemTypeByteSequence, ByteSequence: binaryContent}, inputString, nil
}

// parseBoolean parses a boolean as specified in RFC 9651 section 4.2.8 "Parsing a Boolean".
func parseBoolean(inputString string) (v BareItem, rest string, err error) {
	// https://www.rfc-editor.org/info/rfc9651/#section-4.2.8
	//
	// 1. If the first character of input_string is not "?", fail parsing
	if inputString == "" || inputString[0] != '?' {
		// Note: Unreachable as parseBoolean is only called when we already checked the first character.
		return BareItem{}, "", fmt.Errorf("%w: invalid or missing prefix character", ErrInvalidBoolean)
	}

	// 2. Discard the first character of input_string.
	inputString = inputString[1:]

	// 3. If the first character of input_string matches "1", discard the first character, and return true.
	if inputString != "" && inputString[0] == '1' {
		inputString = inputString[1:]
		return BareItem{Type: BareItemTypeBoolean, Boolean: true}, inputString, nil
	}

	// 4. If the first character of input_string matches "0", discard the first character, and return false.
	if inputString != "" && inputString[0] == '0' {
		inputString = inputString[1:]
		return BareItem{Type: BareItemTypeBoolean, Boolean: false}, inputString, nil
	}

	// 5. No value has matched; fail parsing.
	return BareItem{}, "", ErrInvalidBoolean
}

// parseDate parses a date as specified in RFC 9651 section 4.2.9 "Parsing a Date".
func parseDate(inputString string) (v BareItem, rest string, err error) {
	// From https://www.rfc-editor.org/info/rfc9651/#section-4.2.9
	//
	// 1. If the first character of input_string is not "@", fail parsing.
	if inputString == "" || inputString[0] != '@' {
		// Note: Unreachable as parseDate is only called when we already checked the first character.
		return BareItem{}, "", fmt.Errorf("%w: invalid or missing prefix character", ErrInvalidBoolean)
	}

	// 2. Discard the first character of input_string.
	inputString = inputString[1:]

	// 3. Let output_date be the result of running Parsing an Integer or Decimal (Section 4.2.4) with input_string.
	v, inputString, err = parseIntegerOrDecimal(inputString)
	if err != nil {
		return BareItem{}, "", ErrInvalidDate
	}

	// 4. If output_date is a Decimal, fail parsing.
	if v.Type == BareItemTypeDecimal {
		return BareItem{}, "", ErrInvalidDate
	}

	// 5. Return output_date.
	return BareItem{Type: BareItemTypeDate, Date: v.Integer}, inputString, nil
}

// parseDisplayString parses a display string as specified in RFC 9651 section 4.2.10 "Parsing a Display String".
func parseDisplayString(inputString string) (v BareItem, rest string, err error) {
	// From https://www.rfc-editor.org/info/rfc9651/#section-4.2.10
	//
	// 1. If the first two characters of input_string are not "%" followed by DQUOTE, fail parsing.
	if len(inputString) < 2 || inputString[0] != '%' || inputString[1] != _DQUOTE {
		return BareItem{}, "", fmt.Errorf("%w: invalid or missing prefix characters", ErrInvalidDisplayString)
	}

	// 2. Discard the first two characters of input_string.
	inputString = inputString[2:]

	// 3. Let byte_array be an empty byte array.
	// TODO: Optimize
	var byteArray []byte

	// 4. While input_string is not empty:
	for inputString != "" {
		// 1. Let char be the result of consuming the first character of input_string.
		var char byte
		char, inputString = inputString[0], inputString[1:]

		// 2. If char is in the range %x00-1f or %x7f-ff (i.e., it is not in VCHAR or SP), fail parsing.
		if char <= 0x1f || char >= 0x7f {
			return BareItem{}, "", fmt.Errorf("%w: invalid character", ErrInvalidDisplayString)
		}

		// 3. If char is "%":
		if char == '%' {
			// 1. Let octet_hex be the result of consuming two characters from input_string. If there are not two characters, fail parsing.
			if len(inputString) < 2 {
				return BareItem{}, "", fmt.Errorf("%w: missing characters after %%", ErrInvalidDisplayString)
			}
			var octetHex string
			octetHex, inputString = inputString[:2], inputString[2:]

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
			byteArray = append(byteArray, octet)
			continue
		}

		// 4. If char is DQUOTE:
		if char == _DQUOTE {
			// 1. Let unicode_sequence be the result of decoding byte_array as a UTF-8 string (Section 3 of [UTF8]). Fail parsing if decoding fails.
			if !utf8.Valid(byteArray) {
				return BareItem{}, "", fmt.Errorf("%w: invalid UTF-8 string", ErrInvalidDisplayString)
			}

			unicodeSequence := string(byteArray)

			// 2. Return unicode_sequence:
			return BareItem{Type: BareItemTypeDisplayString, DisplayString: unicodeSequence}, inputString, nil
		}

		// 5. Otherwise, if char is not %" or DQUOTE:
		{
			// 1. Let byte be the result of applying ASCII encoding to char.
			byte_ := char // already ASCII?

			// 2. Append byte to byte_array
			byteArray = append(byteArray, byte_)
		}
	}

	// 5. Reached the end of input_string without finding a closing DQUOTE; fail parsing.
	return BareItem{}, "", fmt.Errorf("%w: missing closing quote", ErrInvalidDisplayString)
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
				value := ItemOrInnerList{
					Type: ItemOrInnerListTypeItem,
					Item: Item{
						BareItem: BareItem{
							Type:    BareItemTypeBoolean,
							Boolean: true,
						},
					},
				}

				// 2. Let parameters be the result of running Parsing Parameters (Section 4.2.3.2) with input_string.
				value.Item.Parameters, inputString, err = parseParameters(inputString)
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
				return InnerList{}, "", fmt.Errorf("%w: %s", ErrInvalidInnerList, err)
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
		innerList.Members = append(innerList.Members, item)

		// 5. If the first character of input_string is not SP or ")", fail parsing.
		if inputString == "" || (inputString[0] != _SP && inputString[0] != ')') {
			return InnerList{}, "", fmt.Errorf("%w: unexpected end of inner list", ErrInvalidInnerList)
		}
	}

	// 4. The end of the Inner List was not found; fail parsing.
	return InnerList{}, "", fmt.Errorf("%w: unexpected end of inner list", ErrInvalidInnerList)
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
		return Item{}, "", err
	}

	// 2. Let parameters be the result of running Parsing Parameters (Section 4.2.3.2) with input_string.
	params, inputString, err := parseParameters(inputString)
	if err != nil {
		return Item{}, "", err
	}

	// 3. Return the tuple (bare_item, parameters).
	return Item{BareItem: item, Parameters: params}, inputString, nil
}

// ItemOrInnerList contains either an [Item] or [InnerList] as part of a [Dictionary] or [List].
//
// It acts as a tagged union, with [ListMember.Type] specifying the type and thus which other field of the struct is set.
type ItemOrInnerList struct {
	// Type is used as tag to denote the field in which the value is stored.
	Type ItemOrInnerListType

	// InnerList contains the value if Type is [ItemOrInnerListTypeInnerList].
	InnerList InnerList

	// Item contains the value if Type is [ItemOrInnerListTypeItem].
	Item Item
}

// ItemOrInnerListType is an enum of types that a ItemOrInnerList can contain.
type ItemOrInnerListType uint8

const (
	// ItemOrInnerListTypeInnerList denotes a ItemOrInnerList containing an InnerList.
	ItemOrInnerListTypeInnerList ItemOrInnerListType = iota

	// ItemOrInnerListTypeItem denotes a ItemOrInnerList containing an Item.
	ItemOrInnerListTypeItem
)

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
		return ItemOrInnerList{Type: ItemOrInnerListTypeInnerList, InnerList: innerList}, inputString, nil
	}

	// 2. Return the result of running Parsing an Item (Section 4.2.3) with input_string.
	item, inputString, err := parseItem(inputString)
	if err != nil {
		return ItemOrInnerList{}, "", err
	}
	return ItemOrInnerList{Type: ItemOrInnerListTypeItem, Item: item}, inputString, nil
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

// ListMemberType is an enum of types that a ListMember can contain.
type ListMemberType uint8

const (
	// ListMemberTypeInnerList denotes a ListMember containing an InnerList.
	ListMemberTypeInnerList ListMemberType = iota

	// ListMemberTypeItem denotes a ListMember containing an Item.
	ListMemberTypeItem
)

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
			return List{}, "", err
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

// String returns the type name, which is the name of the constant minus the type prefix.
func (l ListMemberType) String() string {
	switch l {
	case ListMemberTypeInnerList:
		return "InnerList"
	case ListMemberTypeItem:
		return "Item"
	default:
		panic(fmt.Sprintf("unknown ListMemberType %d", uint8(l)))
	}
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
			return Parameters{}, inputString, fmt.Errorf("failed to parse parameter key: %w", err)
		}

		// 5. Let param_value be Boolean true.
		paramValue := BareItem{Type: BareItemTypeBoolean, Boolean: true}

		// 6. If the first character of input_string is "=":
		if inputString != "" && inputString[0] == '=' {
			// 1. Consume the "=" character at the beginning of input_string.
			inputString = inputString[1:]

			// 2. Let param_value be the result of running Parsing a Bare Item (Section 4.2.3.1) with input_string.
			paramValue, inputString, err = parseBareItem(inputString)
			if err != nil {
				return Parameters{}, inputString, fmt.Errorf("failed to parse parameter value: %w", err)
			}
		}

		// 7. If parameters already contains a key param_key (comparing character for character), overwrite its value with param_value.
		// 8. Otherwise, append key param_key with value param_value to parameters.
		m.Set(paramKey, paramValue)
	}

	// 3. Return parameters.
	return Parameters{m}, inputString, nil
}
