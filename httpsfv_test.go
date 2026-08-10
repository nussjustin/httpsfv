package httpsfv

import (
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var cmpOpts = []cmp.Option{
	cmp.Transformer("BareItem", func(v BareItem) string {
		var val any
		switch v.Type {
		case BareItemTypeInvalid:
		case BareItemTypeBoolean:
			val = v.Boolean
		case BareItemTypeByteSequence:
			val = string(v.ByteSequence)
		case BareItemTypeDate:
			val = v.Date
		case BareItemTypeDecimal:
			val = v.Decimal
		case BareItemTypeDisplayString:
			val = v.DisplayString
		case BareItemTypeInteger:
			val = v.Integer
		case BareItemTypeString:
			val = v.String
		case BareItemTypeToken:
			val = v.Token
		}
		return fmt.Sprintf("%s(%v)", v.Type, val)
	}),
	cmp.Transformer("Dictionary", func(v Dictionary) any {
		type pair struct {
			Key   any
			Value any
		}
		var pairs []pair
		for k, v := range v.All() {
			pairs = append(pairs, pair{k, v})
		}
		return pairs
	}),
	cmp.Transformer("ItemOrInnerList", func(v ItemOrInnerList) any {
		var data struct {
			Type  ItemOrInnerListType
			Value any
		}
		data.Type = v.Type()
		switch data.Type {
		case ItemOrInnerListTypeInnerList:
			data.Value = v.InnerList()
		case ItemOrInnerListTypeItem:
			data.Value = v.Item()
		}
		return data
	}),
	cmp.Transformer("Parameters", func(v Parameters) any {
		type pair struct {
			Key   any
			Value any
		}
		var pairs []pair
		for k, v := range v.All() {
			pairs = append(pairs, pair{k, v})
		}
		return pairs
	}),
}

func dict(args ...any) Dictionary {
	if len(args)%2 != 0 {
		panic("bad number of parameters")
	}

	var p Dictionary

	for i := 0; i < len(args); i += 2 {
		key := args[i].(string)
		var value ItemOrInnerList
		switch v := args[i+1].(type) {
		case InnerList:
			value = ItemOrInnerListFrom(v)
		case Item:
			value = ItemOrInnerListFrom(v)
		default:
			panic(fmt.Sprintf("unexpected value of type %T", v))
		}
		p.Set(key, value)
	}

	return p
}

func members(member ...any) []ItemOrInnerList {
	var ms []ItemOrInnerList
	for _, m := range member {
		switch v := m.(type) {
		case InnerList:
			ms = append(ms, ItemOrInnerListFrom(v))
		case Item:
			ms = append(ms, ItemOrInnerListFrom(v))
		default:
			panic(fmt.Sprintf("unexpected value of type %T", v))
		}
	}
	return ms
}

func params(args ...any) Parameters {
	if len(args)%2 != 0 {
		panic("bad number of parameters")
	}

	var p Parameters

	for i := 0; i < len(args); i += 2 {
		key, value := args[i].(string), args[i+1].(BareItem)
		p.Set(key, value)
	}

	return p
}

func TestParse_Dictionary(t *testing.T) {
	testCases := []parseDictionaryTestCase{
		{
			name:      "non-ascii",
			input:     `"hêllo"`,
			wantError: ErrNonAsciiInput,
		},
		{
			name:      "invalid key",
			input:     `"key=value"`,
			wantError: ErrInvalidKey,
		},
		{
			name:      "invalid bare item",
			input:     `key="hello`, // missing ending quote
			wantError: ErrInvalidBareItem,
		},
	}

	for _, testCase := range parseDictionaryTestCases {
		if testCase.skipForParse {
			continue
		}

		testCaseWithLeadingSpaces := testCase
		testCaseWithLeadingSpaces.name += " - with leading spaces"
		testCaseWithLeadingSpaces.input = "   " + testCaseWithLeadingSpaces.input

		testCaseWithTrailingSpaces := testCase
		testCaseWithTrailingSpaces.name += " - with trailing spaces"
		testCaseWithTrailingSpaces.input += "   "

		testCases = append(testCases,
			testCase,
			testCaseWithLeadingSpaces,
			testCaseWithTrailingSpaces)
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := Parse[Dictionary](testCase.input)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("got error %v, want %v", err, testCase.wantError)
			}

			if diff := cmp.Diff(testCase.want, got, cmpOpts...); diff != "" {
				t.Errorf("Parse[Dictionary]() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParse_Item(t *testing.T) {
	testCases := []parseItemTestCase{
		{
			name:      "non-ascii",
			input:     `"hêllo"`,
			wantError: ErrNonAsciiInput,
		},
		{
			name:      "invalid bare item",
			input:     `"hello`, // missing ending quote
			wantError: ErrInvalidBareItem,
		},
	}

	for _, testCase := range parseItemTestCases {
		if testCase.skipForParse || testCase.wantError != nil {
			continue
		}

		testCaseWithLeadingSpaces := testCase
		testCaseWithLeadingSpaces.name += " - with leading spaces"
		testCaseWithLeadingSpaces.input = "   " + testCaseWithLeadingSpaces.input

		testCaseWithTrailingSpaces := testCase
		testCaseWithTrailingSpaces.name += " - with trailing spaces"
		testCaseWithTrailingSpaces.input += "   "

		testCaseWithTrailingData := testCase
		testCaseWithTrailingData.name += " - with trailing data"
		testCaseWithTrailingData.input += " test"
		testCaseWithTrailingData.want = Item{}
		testCaseWithTrailingData.wantError = ErrTrailingData

		testCases = append(testCases,
			testCase,
			testCaseWithLeadingSpaces,
			testCaseWithTrailingSpaces,
			testCaseWithTrailingData)
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := Parse[Item](testCase.input)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("got error %v, want %v", err, testCase.wantError)
			}

			if diff := cmp.Diff(testCase.want, got, cmpOpts...); diff != "" {
				t.Errorf("Parse[Item]() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParse_List(t *testing.T) {
	testCases := []parseListTestCase{
		{
			name:      "non-ascii",
			input:     `"hêllo"`,
			wantError: ErrNonAsciiInput,
		},
		{
			name:      "invalid bare item",
			input:     `"hello`, // missing ending quote
			wantError: ErrInvalidBareItem,
		},
	}

	for _, testCase := range parseListTestCases {
		if testCase.skipForParse {
			continue
		}

		testCases = append(testCases, testCase)
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := Parse[List](testCase.input)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("got error %v, want %v", err, testCase.wantError)
			}

			if diff := cmp.Diff(testCase.want, got, cmpOpts...); diff != "" {
				t.Errorf("Parse[List]() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func BenchmarkParse(b *testing.B) {
	b.Run("Dictionary", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			_, err := Parse[Dictionary](`a=123,b=123.456,c=(?0 ?1),d=@123456,e=token,f=("string" %"display string" :dGVzdA==:)`)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("List", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			_, err := Parse[List](`123,123.456,(?0 ?1),@123456,token,("string" %"display string" :dGVzdA==:)`)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

var fuzzParseInputs = []string{
	" ",
	"(1,2)",
	"(1\t2)",
	"-",
	"-a",
	"123 a",
	"123.",
	"123.4567",
	"1234567890123.4",
	"1234567890123.456",
	"1234567890123456",
	"123; A=1",
	"123; a=-",
	"123; aB=1",
	"123;",
	":==dGVzdA==:",
	":dGVzdA== :",
	":dGVzdA==",
	"?01",
	"?2",
	"@",
	"@12345678901234567",
	"@a",
	"hêllo",
	`"\a"`,
	`"hello world`,
	`"hello` + string([]byte{0}) + `"`,
	`%"%Cc"`,
	`%"%`,
	`%"%cC"`,
	`%"%c`,
	`%"%ea"`,
	`%"hello world`,
	`%hello world"`,
	`(1 (2 3) 4)`,
	`(123; param=value`,
	`(123`,
	`(123a)`,
	`a =b`,
	`a= b`,
	`a=(b=c)`,
	`a=123a`,
	`a=b,`,
	`hello,`,
	"    123",
	"*hello",
	"-123.456",
	"-123456",
	"123   ",
	"123.456",
	"123.456; param1=1.2;param2=2.3; param1=3.4",
	"123456",
	"123456789012.34",
	"123456789012.345",
	"123456789012345",
	"123; *key_with-many.chars_*=1",
	"123; bool; non-bool=1",
	"123; param1=1;param2=2; param1=3",
	"::",
	":dGVzdA:",
	":dGVzdA==:",
	":dGVzdA==:; param1=:dmFsdWUx:;param2=:dmFsdWUy:; param1=:dmFsdWUz:",
	"?0",
	"?1",
	"?1; param1=?0;param2=?0; param1=?1",
	"@-123456",
	"@123456",
	"@123; param1=@1;param2=@2; param1=@3",
	"Hello",
	"h!#$%&'+-.^_`|~:/",
	"h!#$%&'+-.^_`|~:/",
	"hello",
	"hello; param1=value1;param2=value2; param1=value3",
	` ( 123 123.456 ) , ( ?0  ?1 ) , ( @123456 token ) , ( "string" %"display string" ) `,
	` 123 , 123.456 , ?0 , ?1 , @123456 , token , "string" ` + "\t" + `,` + "\t" + ` %"display string" `,
	`""`,
	`"hello world"; param1="value 1";param2="value 2"; param1="value 3"`,
	`"hello world"`,
	`"with \" and \\"`,
	`%"%c3%bcsers"`,
	`%"hello world"; param1=%"value 1";param2=%"value 2"; param1=%"value 3"`,
	`%"hello world"`,
	`()`,
	`123,123.456,(?0 ?1),@123456,token,("string" %"display string")`,
	`a; param1=value1;param2=value2; param1=value3`,
	`a=123 , b=123.456, c=?0 , d=?1 , e=@123456 , f=token, g="string" ` + "\t" + `,` + "\t" + ` h=%"display string" `,
	`a=123,b=123.456,c=(?0 ?1),d=@123456,e=token,f=("string" %"display string")`,
	`a`,
	`outer; a=1; b=2; a=3, (inner; a=10; b=20; a=30); a=100; b=200; a=300`,
	"@123.456",
}

func FuzzParse_Dictionary(f *testing.F) {
	for _, input := range fuzzParseInputs {
		f.Add(input)
	}

	f.Fuzz(func(t *testing.T, inputString string) {
		_, _ = Parse[Dictionary](inputString)
	})
}

func FuzzParse_Item(f *testing.F) {
	for _, input := range fuzzParseInputs {
		f.Add(input)
	}

	f.Fuzz(func(t *testing.T, inputString string) {
		_, _ = Parse[Item](inputString)
	})
}

func FuzzParse_List(f *testing.F) {
	for _, input := range fuzzParseInputs {
		f.Add(input)
	}

	f.Fuzz(func(t *testing.T, inputString string) {
		_, _ = Parse[List](inputString)
	})
}

func TestParseLines(t *testing.T) {
	testsCases := []struct {
		name      string
		input     []string
		want      List
		wantError error
	}{
		{
			name: "empty",
		},

		{
			name: "single valid line",
			input: []string{
				`hello, "world"`,
			},
			want: List{
				Members: members(
					Item{BareItem: BareItemToken("hello")},
					Item{BareItem: BareItemString("world")},
				),
			},
		},
		{
			name: "single invalid line",
			input: []string{
				`hello, "world`,
			},
			wantError: ErrInvalidString,
		},

		{
			name: "multiple valid lines",
			input: []string{
				`hello, "world"`,
				`how, "are you?"`,
			},
			want: List{
				Members: members(
					Item{BareItem: BareItemToken("hello")},
					Item{BareItem: BareItemString("world")},
					Item{BareItem: BareItemToken("how")},
					Item{BareItem: BareItemString("are you?")},
				),
			},
		},
		{
			name: "multiple lines with one invalid",
			input: []string{
				`hello, "world`,
				`i, just, want, "to ask`,
				`how, "are you?"`,
			},
			wantError: ErrInvalidString,
		},
	}

	for _, testCase := range testsCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := ParseLines[List](testCase.input)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("got error %v, want %v", err, testCase.wantError)
			}

			if diff := cmp.Diff(testCase.want, got, cmpOpts...); diff != "" {
				t.Errorf("ParseLines[List]() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_parseKey(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantKey   string
		wantRest  string
		wantError error
	}{
		{
			name:      "empty input",
			wantError: ErrInvalidKey,
		},
		{
			name:      "empty key",
			input:     `=world`,
			wantError: ErrInvalidKey,
		},
		{
			name:     "valid",
			input:    `hello=world`,
			wantKey:  `hello`,
			wantRest: `=world`,
		},
		{
			name:     "valid starting with asterisk",
			input:    `*hello=world`,
			wantKey:  `*hello`,
			wantRest: `=world`,
		},
		{
			name:     "valid complex",
			input:    `*key_with-many.chars_*=world`,
			wantKey:  `*key_with-many.chars_*`,
			wantRest: `=world`,
		},
		{
			name:      "invalid starting with upper case alpha",
			input:     `Hello=world`,
			wantError: ErrInvalidKey,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			gotKey, gotRest, err := parseKey(testCase.input)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("parseKey() error %v, want %v", err, testCase.wantError)
			}

			if gotKey != testCase.wantKey {
				t.Errorf("parseKey() gotKey = %v, want %v", gotKey, testCase.wantKey)
			}

			if gotRest != testCase.wantRest {
				t.Errorf("parseKey() gotRest = %q, want %q", gotRest, testCase.wantRest)
			}
		})
	}
}

func Test_serializeKey(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      string
		wantError error
	}{
		{
			name:  "lower case alpha only",
			input: `hello`,
			want:  `hello`,
		},
		{
			name:  "star",
			input: `*`,
			want:  `*`,
		},
		{
			name:  "star follow by more",
			input: `*hello`,
			want:  `*hello`,
		},
		{
			name:  "complex",
			input: `*h_1-9.*`,
			want:  `*h_1-9.*`,
		},
		{
			name:      "empty",
			input:     ``,
			wantError: ErrInvalidKey,
		},
		{
			name:      "non-ascii",
			input:     `hällo`,
			wantError: ErrInvalidKey,
		},
		{
			name:      "first character digit",
			input:     `1ello`,
			wantError: ErrInvalidKey,
		},
		{
			name:      "first character upper case alpha",
			input:     `Hello`,
			wantError: ErrInvalidKey,
		},
		{
			name:      "invalid character after first",
			input:     `hEllo`,
			wantError: ErrInvalidKey,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := serializeKey([]byte("prefix: "), testCase.input)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("serializeKey() error %v, want %v", err, testCase.wantError)
			}

			if err != nil {
				return
			}

			want := "prefix: " + testCase.want

			if string(got) != want {
				t.Errorf("serializeKey() got = %q, want %q", string(got), want)
			}
		})
	}
}

type parseBareItemTestCase struct {
	name         string
	input        string
	want         BareItem
	wantRest     string
	wantError    error
	skipForParse bool
}

var parseBareItemTestCases []parseBareItemTestCase

func Test_parseBareItem(t *testing.T) {
	for _, testCase := range parseBareItemTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			gotBareItem, gotRest, err := parseBareItem(testCase.input)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("parseBareItem() error %v, want %v", err, testCase.wantError)
			}

			if diff := cmp.Diff(testCase.want, gotBareItem, cmpOpts...); diff != "" {
				t.Errorf("parseBareItem() mismatch (-want, +got):\n%s", diff)
			}

			if gotRest != testCase.wantRest {
				t.Errorf("parseBareItem() gotRest = %q, want %q", gotRest, testCase.wantRest)
			}
		})
	}
}

type serializeBareItemTestCase struct {
	name      string
	input     BareItem
	want      string
	wantError error
}

var serializeBareItemTestCases []serializeBareItemTestCase

func Test_serializeBareItem(t *testing.T) {
	for _, testCase := range serializeBareItemTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := serializeBareItem([]byte("prefix: "), testCase.input)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("serializeBareItem() error %v, want %v", err, testCase.wantError)
			}

			if err != nil {
				return
			}

			gotStr := string(got)
			want := "prefix: " + testCase.want

			if gotStr != want {
				t.Errorf("serializeBareItem() got = %q, want %q", gotStr, want)
			}
		})
	}
}

func TestBareItem_AppendText(t *testing.T) {
	for _, testCase := range serializeBareItemTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := testCase.input.AppendText([]byte("prefix: "))

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("BareItem.AppendText() error %v, want %v", err, testCase.wantError)
			}

			if err != nil {
				return
			}

			gotStr := string(got)
			want := "prefix: " + testCase.want

			if gotStr != want {
				t.Errorf("BareItem.AppendText() got = %q, want %q", gotStr, want)
			}
		})
	}
}

var parseBareItemIntegerOrDecimalTestCases = []parseBareItemTestCase{
	{
		name:         "empty input",
		wantError:    ErrInvalidIntegerOrDecimal,
		skipForParse: true,
	},
	{
		name:  "decimal",
		input: "123.456",
		want:  BareItemDecimal(123.456),
	},
	{
		name:  "negative decimal",
		input: "-123.456",
		want:  BareItemDecimal(-123.456),
	},
	{
		name:      "decimal with plus sign",
		input:     "+123.456",
		wantError: ErrInvalidIntegerOrDecimal,
	},
	{
		name:  "decimal with exactly 12 characters before dot",
		input: "123456789012.34",
		want:  BareItemDecimal(123456789012.34),
	},
	{
		name:      "decimal with more than 12 characters before dot",
		input:     "1234567890123.4",
		wantError: ErrInvalidIntegerOrDecimal,
	},
	{
		name:  "decimal with exactly 16 characters",
		input: "123456789012.345",
		want:  BareItemDecimal(123456789012.345),
	},
	{
		name:      "decimal with more than 16 characters",
		input:     "1234567890123.456",
		wantError: ErrInvalidIntegerOrDecimal,
	},
	{
		name:      "decimal with dot at end",
		input:     "123.",
		wantError: ErrInvalidIntegerOrDecimal,
	},
	{
		name:      "decimal with more than 3 digits after dot",
		input:     "123.4567",
		wantError: ErrInvalidIntegerOrDecimal,
	},
	{
		name:         "leading spaces before decimal",
		input:        "    123.456",
		wantError:    ErrInvalidIntegerOrDecimal,
		skipForParse: true,
	},
	{
		name:         "trailing spaces after decimal",
		input:        "123.456   ",
		want:         BareItemDecimal(123.456),
		wantRest:     "   ",
		skipForParse: true,
	},
	{
		name:         "trailing data after decimal",
		input:        "123.456abc",
		want:         BareItemDecimal(123.456),
		wantRest:     "abc",
		skipForParse: true,
	},

	{
		name:  "integer",
		input: "123456",
		want:  BareItemInteger(123_456),
	},
	{
		name:  "negative integer",
		input: "-123456",
		want:  BareItemInteger(-123_456),
	},
	{
		name:      "integer with plus sign",
		input:     "+123456",
		wantError: ErrInvalidIntegerOrDecimal,
	},
	{
		name:  "integer with exactly 15 characters",
		input: "123456789012345",
		want:  BareItemInteger(123_456_789_012_345),
	},
	{
		name:      "integer with more than 15 characters",
		input:     "1234567890123456",
		wantError: ErrInvalidIntegerOrDecimal,
	},
	{
		name:      "leading spaces before integer",
		input:     "    123",
		wantError: ErrInvalidIntegerOrDecimal,
	},
	{
		name:         "trailing spaces after integer",
		input:        "123   ",
		want:         BareItemInteger(123),
		wantRest:     "   ",
		skipForParse: true,
	},
	{
		name:         "trailing data after integer",
		input:        "123abc",
		want:         BareItemInteger(123),
		wantRest:     "abc",
		skipForParse: true,
	},
	{
		name:      "lone minus",
		input:     "-",
		wantError: ErrInvalidIntegerOrDecimal,
	},
	{
		name:      "non-digit after minus",
		input:     "-a",
		wantError: ErrInvalidIntegerOrDecimal,
	},
}

func Test_parseBareItemIntegerOrDecimal(t *testing.T) {
	for _, testCase := range parseBareItemIntegerOrDecimalTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			gotBareItem, gotRest, err := parseBareItemIntegerOrDecimal(testCase.input)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("parseBareItemIntegerOrDecimal() error %v, want %v", err, testCase.wantError)
			}

			if diff := cmp.Diff(testCase.want, gotBareItem, cmpOpts...); diff != "" {
				t.Errorf("parseBareItemIntegerOrDecimal() mismatch (-want, +got):\n%s", diff)
			}

			if gotRest != testCase.wantRest {
				t.Errorf("parseBareItemIntegerOrDecimal() gotRest = %q, want %q", gotRest, testCase.wantRest)
			}
		})
	}
}

var serializeBareItemIntegerTestCases = []serializeBareItemTestCase{
	{
		name:  "zero",
		input: BareItemInteger(0),
		want:  "0",
	},
	{
		name:  "positive",
		input: BareItemInteger(123_456),
		want:  "123456",
	},
	{
		name:  "negative",
		input: BareItemInteger(-123_456),
		want:  "-123456",
	},
	{
		name:  "positive at range end",
		input: BareItemInteger(999_999_999_999_999),
		want:  "999999999999999",
	},
	{
		name:  "negative at range end",
		input: BareItemInteger(-999_999_999_999_999),
		want:  "-999999999999999",
	},
	{
		name:      "positive outside range",
		input:     BareItemInteger(999_999_999_999_999 + 1),
		wantError: ErrInvalidIntegerOrDecimal,
	},
	{
		name:      "negative outside range",
		input:     BareItemInteger(-999_999_999_999_999 - 1),
		wantError: ErrInvalidIntegerOrDecimal,
	},
}

func Test_serializeBareItemInteger(t *testing.T) {
	for _, testCase := range serializeBareItemIntegerTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := serializeBareItemInteger([]byte("prefix: "), testCase.input.Integer)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("serializeBareItemInteger() error %v, want %v", err, testCase.wantError)
			}

			if err != nil {
				return
			}

			want := "prefix: " + testCase.want

			if string(got) != want {
				t.Errorf("serializeBareItemInteger() got = %q, want %q", string(got), want)
			}
		})
	}
}

var serializeBareItemDecimalTestCases = []serializeBareItemTestCase{
	{
		name:  "zero",
		input: BareItemDecimal(0),
		want:  "0.0",
	},
	{
		name:      "NaN",
		input:     BareItemDecimal(math.NaN()),
		wantError: ErrInvalidIntegerOrDecimal,
	},
	{
		name:      "+Inf",
		input:     BareItemDecimal(math.Inf(0)),
		wantError: ErrInvalidIntegerOrDecimal,
	},
	{
		name:      "-Inf",
		input:     BareItemDecimal(math.Inf(1)),
		wantError: ErrInvalidIntegerOrDecimal,
	},
	{
		name:  "positive",
		input: BareItemDecimal(123_456.0),
		want:  "123456.0",
	},
	{
		name:  "negative",
		input: BareItemDecimal(-123_456.0),
		want:  "-123456.0",
	},
	{
		name:  "positive with full precision",
		input: BareItemDecimal(123_456.789),
		want:  "123456.789",
	},
	{
		name:  "negative with full precision",
		input: BareItemDecimal(-123_456.789),
		want:  "-123456.789",
	},
	{
		name:  "positive rounded up",
		input: BareItemDecimal(123_456.7895),
		want:  "123456.79",
	},
	{
		name:  "negative rounded up",
		input: BareItemDecimal(-123_456.7895),
		want:  "-123456.79",
	},
	{
		name:  "positive rounded down",
		input: BareItemDecimal(123_456.7894),
		want:  "123456.789",
	},
	{
		name:  "negative rounded down",
		input: BareItemDecimal(-123_456.7894),
		want:  "-123456.789",
	},
	{
		name:  "positive at range end",
		input: BareItemDecimal(999_999_999_999.0),
		want:  "999999999999.0",
	},
	{
		name:  "negative at range end",
		input: BareItemDecimal(-999_999_999_999.0),
		want:  "-999999999999.0",
	},
	{
		name:      "positive outside range",
		input:     BareItemDecimal(999_999_999_999 + 1),
		wantError: ErrInvalidIntegerOrDecimal,
	},
	{
		name:      "negative outside range",
		input:     BareItemDecimal(-999_999_999_999 - 1),
		wantError: ErrInvalidIntegerOrDecimal,
	},
}

func Test_serializeBareItemDecimal(t *testing.T) {
	for _, testCase := range serializeBareItemDecimalTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := serializeBareItemDecimal([]byte("prefix: "), testCase.input.Decimal)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("serializeBareItemDecimal() error %v, want %v", err, testCase.wantError)
			}

			if err != nil {
				return
			}

			want := "prefix: " + testCase.want

			if string(got) != want {
				t.Errorf("serializeBareItemDecimal() got = %q, want %q", string(got), want)
			}
		})
	}
}

var parseBareItemStringTestCases = []parseBareItemTestCase{
	{
		name:         "empty input",
		wantError:    ErrInvalidString,
		skipForParse: true,
	},
	{
		name:  "empty",
		input: `""`,
		want:  BareItemString(``),
	},
	{
		name:  "simple",
		input: `"hello world"`,
		want:  BareItemString(`hello world`),
	},
	{
		name:  "with escaped characters",
		input: `"with \" and \\"`,
		want:  BareItemString(`with " and \`),
	},
	{
		name:      "with invalid escape",
		input:     `"\a"`,
		wantError: ErrInvalidString,
	},
	{
		name:      "with invalid character",
		input:     `"hello` + string([]byte{0}) + `"`,
		wantError: ErrInvalidString,
	},
	{
		name:      "without ending quote",
		input:     `"hello world`,
		wantError: ErrInvalidString,
	},
	{
		name:      "wrong quotes",
		input:     `'hello world'`,
		wantError: ErrInvalidString,
	},
	{
		name:         "with leading spaces",
		input:        `   "hello world"`,
		wantError:    ErrInvalidString,
		skipForParse: true,
	},
	{
		name:         "with trailing spaces",
		input:        `"hello world"   `,
		want:         BareItemString(`hello world`),
		wantRest:     `   `,
		skipForParse: true,
	},
	{
		name:         "with trailing data",
		input:        `"hello world"abc`,
		want:         BareItemString(`hello world`),
		wantRest:     `abc`,
		skipForParse: true,
	},
}

func Test_parseBareItemString(t *testing.T) {
	for _, testCase := range parseBareItemStringTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			gotBareItem, gotRest, err := parseBareItemString(testCase.input)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("parseBareItemString() error %v, want %v", err, testCase.wantError)
			}

			if diff := cmp.Diff(testCase.want, gotBareItem, cmpOpts...); diff != "" {
				t.Errorf("parseBareItemString() mismatch (-want, +got):\n%s", diff)
			}

			if gotRest != testCase.wantRest {
				t.Errorf("parseBareItemString() gotRest = %q, want %q", gotRest, testCase.wantRest)
			}
		})
	}
}

var serializeBareItemStringTestCases = []serializeBareItemTestCase{
	{
		name:  "empty",
		input: BareItemString(``),
		want:  `""`,
	},
	{
		name:  "simple",
		input: BareItemString(`hello world`),
		want:  `"hello world"`,
	},
	{
		name:  "with backslashes",
		input: BareItemString(`\a \ will be escaped\`),
		want:  `"\\a \\ will be escaped\\"`,
	},
	{
		name:  "with quotes",
		input: BareItemString(`"hello "awesome" world"`),
		want:  `"\"hello \"awesome\" world\""`,
	},
	{
		name:  "with backslashes and quotes",
		input: BareItemString(`with \ and "`),
		want:  `"with \\ and \""`,
	},
	{
		name:      "non-ascii",
		input:     BareItemString("\u26a1"),
		wantError: ErrInvalidString,
	},
}

func Test_serializeBareItemString(t *testing.T) {
	for _, testCase := range serializeBareItemStringTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := serializeBareItemString([]byte("prefix: "), testCase.input.String)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("serializeBareItemString() error %v, want %v", err, testCase.wantError)
			}

			if err != nil {
				return
			}

			want := "prefix: " + testCase.want

			if string(got) != want {
				t.Errorf("serializeBareItemString() got = %q, want %q", string(got), want)
			}
		})
	}
}

var parseBareItemTokenTestCases = []parseBareItemTestCase{
	{
		name:         "empty input",
		wantError:    ErrInvalidToken,
		skipForParse: true,
	},
	{
		name:  "simple",
		input: "hello",
		want:  BareItemToken("hello"),
	},
	{
		name:  "starting with upper case character",
		input: "Hello",
		want:  BareItemToken("Hello"),
	},
	{
		name:  "starting with *",
		input: "*hello",
		want:  BareItemToken("*hello"),
	},
	{
		name:  "with complex characters",
		input: "h!#$%&'+-.^_`|~:/",
		want:  BareItemToken("h!#$%&'+-.^_`|~:/"),
	},
	{
		name:  "with invalid first character",
		input: "h!#$%&'+-.^_`|~:/",
		want:  BareItemToken("h!#$%&'+-.^_`|~:/"),
	},
	{
		name:      "quoted",
		input:     `"hello"`,
		wantError: ErrInvalidToken,
	},
	{
		name:         "with leading spaces",
		input:        "   hello",
		wantError:    ErrInvalidToken,
		skipForParse: true,
	},
	{
		name:         "with trailing spaces",
		input:        "hello   ",
		want:         BareItemToken("hello"),
		wantRest:     "   ",
		skipForParse: true,
	},
	{
		name:         "with trailing data",
		input:        "hello(x)",
		want:         BareItemToken("hello"),
		wantRest:     "(x)",
		skipForParse: true,
	},
}

func Test_parseBareItemToken(t *testing.T) {
	for _, testCase := range parseBareItemTokenTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			gotBareItem, gotRest, err := parseBareItemToken(testCase.input)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("parseBareItemToken() error %v, want %v", err, testCase.wantError)
			}

			if diff := cmp.Diff(testCase.want, gotBareItem, cmpOpts...); diff != "" {
				t.Errorf("parseBareItemToken() mismatch (-want, +got):\n%s", diff)
			}

			if gotRest != testCase.wantRest {
				t.Errorf("parseBareItemToken() gotRest = %q, want %q", gotRest, testCase.wantRest)
			}
		})
	}
}

var serializeBareItemTokenTestCases = []serializeBareItemTestCase{
	{
		name:  "simple lower case",
		input: BareItemToken(`hello`),
		want:  `hello`,
	},
	{
		name:  "simple upper case",
		input: BareItemToken(`HELLO`),
		want:  `HELLO`,
	},
	{
		name:  "simple mixed case",
		input: BareItemToken(`HeLlO`),
		want:  `HeLlO`,
	},
	{
		name:  "star",
		input: BareItemToken(`*`),
		want:  `*`,
	},
	{
		name:  "star followed by more",
		input: BareItemToken(`*hello*`),
		want:  `*hello*`,
	},
	{
		name:  "complex",
		input: BareItemToken(`*hello:complex/token*`),
		want:  `*hello:complex/token*`,
	},
	{
		name:      "empty",
		input:     BareItemToken(``),
		wantError: ErrInvalidToken,
	},
	{
		name:      "invalid start",
		input:     BareItemToken(`\`),
		wantError: ErrInvalidToken,
	}, {
		name:      "invalid characters",
		input:     BareItemToken(`a b`),
		wantError: ErrInvalidToken,
	},
	{
		name:      "non-ascii",
		input:     BareItemToken("\u26a1"),
		wantError: ErrInvalidToken,
	},
}

func Test_serializeBareItemToken(t *testing.T) {
	for _, testCase := range serializeBareItemTokenTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := serializeBareItemToken([]byte("prefix: "), testCase.input.Token)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("serializeBareItemToken() error %v, want %v", err, testCase.wantError)
			}

			if err != nil {
				return
			}

			want := "prefix: " + testCase.want

			if string(got) != want {
				t.Errorf("serializeBareItemToken() got = %q, want %q", string(got), want)
			}
		})
	}
}

var parseBareItemByteSequenceTestCases = []parseBareItemTestCase{
	{
		name:         "empty input",
		wantError:    ErrInvalidByteSequence,
		skipForParse: true,
	},
	{
		name:  "valid",
		input: ":dGVzdA==:",
		want:  BareItemByteSequence([]byte("test")),
	},
	{
		name:      "without prefix",
		input:     "dGVzdA==:",
		wantError: ErrInvalidByteSequence,
	},
	{
		name:      "wrong prefix",
		input:     ";dGVzdA==:",
		wantError: ErrInvalidByteSequence,
	},
	{
		name:      "without suffix :",
		input:     ":dGVzdA==",
		wantError: ErrInvalidByteSequence,
	},
	{
		name:      "wrong suffix",
		input:     ":dGVzdA==;",
		wantError: ErrInvalidByteSequence,
	},
	{
		name:  "without padding",
		input: ":dGVzdA:",
		want:  BareItemByteSequence([]byte("test")),
	},
	{
		name:  "without data",
		input: "::",
		want:  BareItemByteSequence([]byte{}),
	},
	{
		name:      "with invalid characters",
		input:     ":dGVzdA== :",
		wantError: ErrInvalidByteSequence,
	},
	{
		name:      "with invalid base64",
		input:     ":==dGVzdA==:",
		wantError: ErrInvalidByteSequence,
	},
	{
		name:         "with leading spaces",
		input:        "   :dGVzdA==:",
		wantError:    ErrInvalidByteSequence,
		skipForParse: true,
	},
	{
		name:         "with trailing spaces",
		input:        ":dGVzdA==:   ",
		want:         BareItemByteSequence([]byte("test")),
		wantRest:     "   ",
		skipForParse: true,
	},
	{
		name:         "with trailing data",
		input:        ":dGVzdA==::x:",
		want:         BareItemByteSequence([]byte("test")),
		wantRest:     ":x:",
		skipForParse: true,
	},
}

func Test_parseBareItemByteSequence(t *testing.T) {
	for _, testCase := range parseBareItemByteSequenceTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			gotBareItem, gotRest, err := parseBareItemByteSequence(testCase.input)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("parseBareItemByteSequence() error %v, want %v", err, testCase.wantError)
			}

			if diff := cmp.Diff(testCase.want, gotBareItem, cmpOpts...); diff != "" {
				t.Errorf("parseBareItemByteSequence() mismatch (-want, +got):\n%s", diff)
			}

			if gotRest != testCase.wantRest {
				t.Errorf("parseBareItemByteSequence() gotRest = %q, want %q", gotRest, testCase.wantRest)
			}
		})
	}
}

var serializeBareItemByteSequenceTestCases = []serializeBareItemTestCase{
	{
		name:  "empty",
		input: BareItemByteSequence([]byte(``)),
		want:  `::`,
	},
	{
		name:  "non-empty",
		input: BareItemByteSequence([]byte(`hello world`)),
		want:  `:aGVsbG8gd29ybGQ=:`,
	},
	{
		name:  "unicode",
		input: BareItemByteSequence([]byte(`hällo wörld`)),
		want:  `:aMOkbGxvIHfDtnJsZA==:`,
	},
}

func Test_serializeBareItemByteSequence(t *testing.T) {
	for _, testCase := range serializeBareItemByteSequenceTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := serializeBareItemByteSequence([]byte("prefix: "), testCase.input.ByteSequence)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("serializeBareItemByteSequence() error %v, want %v", err, testCase.wantError)
			}

			if err != nil {
				return
			}

			want := "prefix: " + testCase.want

			if string(got) != want {
				t.Errorf("serializeBareItemByteSequence() got = %q, want %q", string(got), want)
			}
		})
	}
}

var parseBareItemBooleanTestCases = []parseBareItemTestCase{
	{
		name:         "empty input",
		wantError:    ErrInvalidBoolean,
		skipForParse: true,
	},
	{
		name:  "boolean true",
		input: "?1",
		want:  BareItemBoolean(true),
	},
	{
		name:  "boolean false",
		input: "?0",
		want:  BareItemBoolean(false),
	},
	{
		name:      "without prefix",
		input:     "1",
		wantError: ErrInvalidBoolean,
	},
	{
		name:      "wrong prefix",
		input:     "@1",
		wantError: ErrInvalidBoolean,
	},
	{
		name:      "invalid boolean number",
		input:     "?2",
		wantError: ErrInvalidBoolean,
	},
	{
		name:         "with leading spaces",
		input:        "   ?01",
		wantError:    ErrInvalidBoolean,
		skipForParse: true,
	},
	{
		name:         "with trailing spaces",
		input:        "?0   ",
		want:         BareItemBoolean(false),
		wantRest:     "   ",
		skipForParse: true,
	},
	{
		name:         "with trailing data",
		input:        "?01234",
		want:         BareItemBoolean(false),
		wantRest:     "1234",
		skipForParse: true,
	},
}

func Test_parseBareItemBoolean(t *testing.T) {
	for _, testCase := range parseBareItemBooleanTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			gotBareItem, gotRest, err := parseBareItemBoolean(testCase.input)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("parseBareItemBoolean() error %v, want %v", err, testCase.wantError)
			}

			if diff := cmp.Diff(testCase.want, gotBareItem, cmpOpts...); diff != "" {
				t.Errorf("parseBareItemBoolean() mismatch (-want, +got):\n%s", diff)
			}

			if gotRest != testCase.wantRest {
				t.Errorf("parseBareItemBoolean() gotRest = %q, want %q", gotRest, testCase.wantRest)
			}
		})
	}
}

var serializeBareItemBooleanTestCases = []serializeBareItemTestCase{
	{
		name:  "false",
		input: BareItemBoolean(false),
		want:  `?0`,
	},
	{
		name:  "true",
		input: BareItemBoolean(true),
		want:  `?1`,
	},
}

func Test_serializeBareItemBoolean(t *testing.T) {
	for _, testCase := range serializeBareItemBooleanTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := serializeBareItemBoolean([]byte("prefix: "), testCase.input.Boolean)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("serializeBareItemBoolean() error %v, want %v", err, testCase.wantError)
			}

			if err != nil {
				return
			}

			want := "prefix: " + testCase.want

			if string(got) != want {
				t.Errorf("serializeBareItemBoolean() got = %q, want %q", string(got), want)
			}
		})
	}
}

var parseBareItemDateTestCases = []parseBareItemTestCase{
	{
		name:         "empty input",
		wantError:    ErrInvalidDate,
		skipForParse: true,
	},
	{
		name:  "positive",
		input: "@123456",
		want:  BareItemDate(123456),
	},
	{
		name:  "negative date",
		input: "@-123456",
		want:  BareItemDate(-123456),
	},
	{
		name:      "without prefix",
		input:     "123456",
		wantError: ErrInvalidDate,
	},
	{
		name:      "wrong prefix",
		input:     "?123456",
		wantError: ErrInvalidDate,
	},
	{
		name:      "missing number after @",
		input:     "@",
		wantError: ErrInvalidDate,
	},
	{
		name:      "non-digit after @",
		input:     "@a",
		wantError: ErrInvalidDate,
	},
	{
		name:      "decimal",
		input:     "@123.456",
		wantError: ErrInvalidDate,
	},
	{
		name:      "with more than 16 characters",
		input:     "@12345678901234567",
		wantError: ErrInvalidDate,
	},
	{
		name:         "with leading spaces",
		input:        "   @123456",
		wantError:    ErrInvalidDate,
		skipForParse: true,
	},
	{
		name:         "with trailing spaces",
		input:        "@123456   ",
		want:         BareItemDate(123456),
		wantRest:     "   ",
		skipForParse: true,
	},
	{
		name:         "with trailing data",
		input:        "@123456abc",
		want:         BareItemDate(123456),
		wantRest:     "abc",
		skipForParse: true,
	},
}

func Test_parseBareItemDate(t *testing.T) {
	for _, testCase := range parseBareItemDateTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			gotBareItem, gotRest, err := parseBareItemDate(testCase.input)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("parseBareItemDate() error %v, want %v", err, testCase.wantError)
			}

			if diff := cmp.Diff(testCase.want, gotBareItem, cmpOpts...); diff != "" {
				t.Errorf("parseBareItemDate() mismatch (-want, +got):\n%s", diff)
			}

			if gotRest != testCase.wantRest {
				t.Errorf("parseBareItemDate() gotRest = %q, want %q", gotRest, testCase.wantRest)
			}
		})
	}
}

var serializeBareItemDateTestCases = []serializeBareItemTestCase{
	{
		name:  "zero",
		input: BareItemDate(0),
		want:  "@0",
	},
	{
		name:  "positive",
		input: BareItemDate(123_456),
		want:  "@123456",
	},
	{
		name:  "negative",
		input: BareItemDate(-123_456),
		want:  "@-123456",
	},
	{
		name:  "positive at range end",
		input: BareItemDate(999_999_999_999_999),
		want:  "@999999999999999",
	},
	{
		name:  "negative at range end",
		input: BareItemDate(-999_999_999_999_999),
		want:  "@-999999999999999",
	},
	{
		name:      "positive outside range",
		input:     BareItemDate(999_999_999_999_999 + 1),
		wantError: ErrInvalidIntegerOrDecimal,
	},
	{
		name:      "negative outside range",
		input:     BareItemDate(-999_999_999_999_999 - 1),
		wantError: ErrInvalidIntegerOrDecimal,
	},
}

func Test_serializeBareItemDate(t *testing.T) {
	for _, testCase := range serializeBareItemDateTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := serializeBareItemDate([]byte("prefix: "), testCase.input.Date)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("serializeBareItemDate() error %v, want %v", err, testCase.wantError)
			}

			if err != nil {
				return
			}

			want := "prefix: " + testCase.want

			if string(got) != want {
				t.Errorf("serializeBareItemDate() got = %q, want %q", string(got), want)
			}
		})
	}
}

var parseBareItemDisplayStringTestCases = []parseBareItemTestCase{
	{
		name:         "empty input",
		wantError:    ErrInvalidDisplayString,
		skipForParse: true,
	},
	{
		name:  "valid",
		input: `%"hello world"`,
		want:  BareItemDisplayString(`hello world`),
	},
	{
		name:  "with hex",
		input: `%"%c3%bcsers"`,
		want:  BareItemDisplayString(`üsers`),
	},
	{
		name:      "with missing starting quote",
		input:     `%hello world"`,
		wantError: ErrInvalidDisplayString,
	},
	{
		name:      "with missing ending quote",
		input:     `%"hello world`,
		wantError: ErrInvalidDisplayString,
	},
	{
		name:      "with missing hex character 1",
		input:     `%"%`,
		wantError: ErrInvalidDisplayString,
	},
	{
		name:      "with missing hex character 2",
		input:     `%"%c`,
		wantError: ErrInvalidDisplayString,
	},
	{
		name:      "with invalid hex character 1",
		input:     `%"%Cc"`,
		wantError: ErrInvalidDisplayString,
	},
	{
		name:      "invalid hex character 2",
		input:     `%"%cC"`,
		wantError: ErrInvalidDisplayString,
	},
	{
		name:      "with invalid UTF-8",
		input:     `%"%ea"`,
		wantError: ErrInvalidDisplayString,
	},
	{
		name:         "with leading spaces",
		input:        `   %"hello"`,
		wantError:    ErrInvalidDisplayString,
		skipForParse: true,
	},
	{
		name:         "with trailing spaces",
		input:        `%"hello world"   `,
		want:         BareItemDisplayString(`hello world`),
		wantRest:     "   ",
		skipForParse: true,
	},
	{
		name:         "with trailing data",
		input:        `%"hello world"%"test"`,
		want:         BareItemDisplayString(`hello world`),
		wantRest:     `%"test"`,
		skipForParse: true,
	},
}

func Test_parseBareItemDisplayString(t *testing.T) {
	for _, testCase := range parseBareItemDisplayStringTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			gotBareItem, gotRest, err := parseBareItemDisplayString(testCase.input)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("parseBareItemDisplayString() error %v, want %v", err, testCase.wantError)
			}

			if diff := cmp.Diff(testCase.want, gotBareItem, cmpOpts...); diff != "" {
				t.Errorf("parseBareItemDisplayString() mismatch (-want, +got):\n%s", diff)
			}

			if gotRest != testCase.wantRest {
				t.Errorf("parseBareItemDisplayString() gotRest = %q, want %q", gotRest, testCase.wantRest)
			}
		})
	}
}

var serializeBareItemDisplayStringTestCases = []serializeBareItemTestCase{
	{
		name:  "empty",
		input: BareItemDisplayString(``),
		want:  `%""`,
	},
	{
		name:  "simple",
		input: BareItemDisplayString(`hello world`),
		want:  `%"hello world"`,
	},
	{
		name:  "unicode",
		input: BareItemDisplayString(`üsers`),
		want:  `%"%c3%bcsers"`,
	},
	{
		name:  "with backslashes",
		input: BareItemDisplayString(`unescaped \`),
		want:  `%"unescaped \"`,
	},
	{
		name:  "with percent sign",
		input: BareItemDisplayString(`escaped %`),
		want:  `%"escaped %25"`,
	},
	{
		name:  "with quotes",
		input: BareItemDisplayString(`escaped "`),
		want:  `%"escaped %22"`,
	},
	{
		name:      "invalid unicode",
		input:     BareItemDisplayString(string([]byte{0b10000000})),
		wantError: ErrInvalidDisplayString,
	},
	{
		name:      "invalid utf-8",
		input:     BareItemDisplayString(string([]byte{0xdf, 0xff})),
		wantError: ErrInvalidDisplayString,
	},
}

func Test_serializeBareItemDisplayString(t *testing.T) {
	for _, testCase := range serializeBareItemDisplayStringTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := serializeBareItemDisplayString([]byte("prefix: "), testCase.input.DisplayString)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("serializeBareItemDisplayString() error %v, want %v", err, testCase.wantError)
			}

			if err != nil {
				return
			}

			want := "prefix: " + testCase.want

			if string(got) != want {
				t.Errorf("serializeBareItemDisplayString() got = %q, want %q", string(got), want)
			}
		})
	}
}

type parseDictionaryTestCase struct {
	name         string
	input        string
	want         Dictionary
	wantRest     string
	wantError    error
	skipForParse bool
}

var parseDictionaryTestCases = []parseDictionaryTestCase{
	{
		name:         "empty",
		skipForParse: true,
	},
	{
		name:  "dictionary of items",
		input: `a=123 , b=123.456, c=?0 , d=?1 , e=@123456 , f=token, g="string" ` + "\t" + `,` + "\t" + ` h=%"display string" `,
		want: dict(
			"a", Item{BareItem: BareItemInteger(123)},
			"b", Item{BareItem: BareItemDecimal(123.456)},
			"c", Item{BareItem: BareItemBoolean(false)},
			"d", Item{BareItem: BareItemBoolean(true)},
			"e", Item{BareItem: BareItemDate(123456)},
			"f", Item{BareItem: BareItemToken("token")},
			"g", Item{BareItem: BareItemString("string")},
			"h", Item{BareItem: BareItemDisplayString("display string")},
		),
	},
	{
		name:  "mixed dictionary",
		input: `a=123,b=123.456,c=(?0 ?1),d=@123456,e=token,f=("string" %"display string")`,
		want: dict(
			"a", Item{BareItem: BareItemInteger(123)},
			"b", Item{BareItem: BareItemDecimal(123.456)},
			"c", InnerList{
				Members: []Item{
					{BareItem: BareItemBoolean(false)},
					{BareItem: BareItemBoolean(true)},
				},
			},
			"d", Item{BareItem: BareItemDate(123456)},
			"e", Item{BareItem: BareItemToken("token")},
			"f", InnerList{
				Members: []Item{
					{BareItem: BareItemString("string")},
					{BareItem: BareItemDisplayString("display string")},
				},
			},
		),
	},
	{
		name:  "boolean without value",
		input: `a`,
		want: dict(
			"a", Item{BareItem: BareItemBoolean(true)},
		),
	},
	{
		name:  "boolean without value but with parameters",
		input: `a; param1=value1;param2=value2; param1=value3`,
		want: dict(
			"a", Item{
				BareItem: BareItemBoolean(true),
				Parameters: params(
					"param1", BareItemToken("value3"),
					"param2", BareItemToken("value2"),
				),
			},
		),
	},
	{
		name:      "space after =",
		input:     `a= b`,
		wantError: ErrInvalidBareItem,
	},
	{
		name:      "space before =",
		input:     `a =b`,
		wantError: ErrInvalidDictionary,
	},
	{
		name:      "nested dictionaries",
		input:     `a=(b=c)`,
		wantError: ErrInvalidInnerList,
	},
	{
		name:      "trailing comma",
		input:     `a=b,`,
		wantError: ErrInvalidDictionary,
	},
	{
		name:      "invalid character after item",
		input:     `a=123a`,
		wantError: ErrInvalidDictionary,
	},
}

func Test_parseDictionary(t *testing.T) {
	for _, testCase := range parseDictionaryTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			gotDictionary, gotRest, err := parseDictionary(testCase.input)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("parseDictionary() error %v, want %v", err, testCase.wantError)
			}

			if diff := cmp.Diff(testCase.want, gotDictionary, cmpOpts...); diff != "" {
				t.Errorf("parseDictionary() mismatch (-want, +got):\n%s", diff)
			}

			if gotRest != testCase.wantRest {
				t.Errorf("parseDictionary() gotRest = %q, want %q", gotRest, testCase.wantRest)
			}
		})
	}
}

type serializeDictionaryTestCase struct {
	name      string
	input     Dictionary
	want      string
	wantError error
}

var serializeDictionaryTestCases = []serializeDictionaryTestCase{
	{
		name: "empty",
		want: ``,
	},
	{
		name: "single inner list",
		input: dict(
			"key1", InnerList{Members: []Item{
				{
					BareItem: BareItemToken(`token`),
					Parameters: params(
						"param1", BareItemToken("value1"),
						"param2", BareItemToken("value2"),
					),
				},
			}},
		),
		want: `key1=(token;param1=value1;param2=value2)`,
	},
	{
		name: "single item",
		input: dict(
			"key1", Item{
				BareItem: BareItemToken(`token`),
				Parameters: params(
					"param1", BareItemToken("value1"),
					"param2", BareItemToken("value2"),
				),
			},
		),
		want: `key1=token;param1=value1;param2=value2`,
	},
	{
		name: "boolean false",
		input: dict(
			"key1", Item{BareItem: BareItemBoolean(false)},
		),
		want: `key1=?0`,
	},
	{
		name: "boolean true",
		input: dict(
			"key1", Item{BareItem: BareItemBoolean(true)},
		),
		want: `key1`,
	},
	{
		name: "mixed dictionary",
		input: dict(
			"a", Item{BareItem: BareItemInteger(123)},
			"b", Item{BareItem: BareItemDecimal(123.456)},
			"c", InnerList{
				Members: []Item{
					{BareItem: BareItemBoolean(false)},
					{BareItem: BareItemBoolean(true)},
				},
			},
			"d", Item{BareItem: BareItemDate(123456)},
			"e", Item{BareItem: BareItemToken("token")},
			"f", InnerList{
				Members: []Item{
					{BareItem: BareItemString("string")},
					{BareItem: BareItemDisplayString("display string")},
				},
			},
		),
		want: `a=123, b=123.456, c=(?0 ?1), d=@123456, e=token, f=("string" %"display string")`,
	},
	{
		name: "invalid key",
		input: dict(
			"", Item{BareItem: BareItemToken(`token`)},
		),
		wantError: ErrInvalidDictionary,
	},
	{
		name: "invalid inner list",
		input: dict(
			"key1", InnerList{Members: []Item{
				{BareItem: BareItemToken("")},
			}},
		),
		wantError: ErrInvalidDictionary,
	},
	{
		name: "invalid item",
		input: dict(
			"key1", Item{BareItem: BareItemToken("")},
		),
		wantError: ErrInvalidDictionary,
	},
}

func Test_serializeDictionary(t *testing.T) {
	for _, testCase := range serializeDictionaryTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := serializeDictionary([]byte("prefix: "), testCase.input)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("serializeDictionary() error %v, want %v", err, testCase.wantError)
			}

			if err != nil {
				return
			}

			want := "prefix: " + testCase.want

			if string(got) != want {
				t.Errorf("serializeDictionary() got = %q, want %q", got, want)
			}
		})
	}
}

func TestDictionary_AppendText(t *testing.T) {
	for _, testCase := range serializeDictionaryTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := testCase.input.AppendText([]byte("prefix: "))

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("Dictionary.AppendText() error %v, want %v", err, testCase.wantError)
			}

			if err != nil {
				return
			}

			want := "prefix: " + testCase.want

			if string(got) != want {
				t.Errorf("Dictionary.AppendText() got = %q, want %q", got, want)
			}
		})
	}
}

type parseInnerListTestCase struct {
	name      string
	input     string
	want      InnerList
	wantRest  string
	wantError error
}

var parseInnerListTestCases = []parseInnerListTestCase{
	{
		name:      "empty input",
		wantError: ErrInvalidInnerList,
	},
	{
		name:  "empty",
		input: `()`,
	},
	{
		name:  "only spaces",
		input: `( )`,
	},
	{
		name:  "all items",
		input: `( 123 123.456 ?0 ?1 @123456 token "string" %"display string" :dGVzdA==: )`,
		want: InnerList{
			Members: []Item{
				{BareItem: BareItemInteger(123)},
				{BareItem: BareItemDecimal(123.456)},
				{BareItem: BareItemBoolean(false)},
				{BareItem: BareItemBoolean(true)},
				{BareItem: BareItemDate(123456)},
				{BareItem: BareItemToken("token")},
				{BareItem: BareItemString("string")},
				{BareItem: BareItemDisplayString("display string")},
				{BareItem: BareItemByteSequence([]byte("test"))},
			},
		},
	},
	{
		name:  "all items with parameters",
		input: `( 123; a=1 123.456; b=2 ?0; c=3 ?1; d=4 @123456; e=5 token; f=6 "string"; g=7 %"display string"; h=8 :dGVzdA==:; i=9 )`,
		want: InnerList{
			Members: []Item{
				{
					BareItem:   BareItemInteger(123),
					Parameters: params("a", BareItemInteger(1)),
				},
				{
					BareItem:   BareItemDecimal(123.456),
					Parameters: params("b", BareItemInteger(2)),
				},
				{
					BareItem:   BareItemBoolean(false),
					Parameters: params("c", BareItemInteger(3)),
				},
				{
					BareItem:   BareItemBoolean(true),
					Parameters: params("d", BareItemInteger(4)),
				},
				{
					BareItem:   BareItemDate(123456),
					Parameters: params("e", BareItemInteger(5)),
				},
				{
					BareItem:   BareItemToken("token"),
					Parameters: params("f", BareItemInteger(6)),
				},
				{
					BareItem:   BareItemString("string"),
					Parameters: params("g", BareItemInteger(7)),
				},
				{
					BareItem:   BareItemDisplayString("display string"),
					Parameters: params("h", BareItemInteger(8)),
				},
				{
					BareItem:   BareItemByteSequence([]byte("test")),
					Parameters: params("i", BareItemInteger(9)),
				},
			},
		},
	},
}

func Test_parseInnerList(t *testing.T) {
	for _, testCase := range parseInnerListTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			gotInnerList, gotRest, err := parseInnerList(testCase.input)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("parseInnerList() error %v, want %v", err, testCase.wantError)
			}

			if diff := cmp.Diff(testCase.want, gotInnerList, cmpOpts...); diff != "" {
				t.Errorf("parseInnerList() mismatch (-want, +got):\n%s", diff)
			}

			if gotRest != testCase.wantRest {
				t.Errorf("parseInnerList() gotRest = %q, want %q", gotRest, testCase.wantRest)
			}
		})
	}
}

type serializeInnerListTestCase struct {
	name      string
	input     InnerList
	want      string
	wantError error
}

var serializeInnerListTestCases = []serializeInnerListTestCase{
	{
		name: "empty",
		want: `()`,
	},
	{
		name: "single member",
		input: InnerList{Members: []Item{
			{BareItem: BareItemToken(`token`)},
		}},
		want: `(token)`,
	},
	{
		name: "multiple members",
		input: InnerList{
			Members: []Item{
				{BareItem: BareItemInteger(123)},
				{BareItem: BareItemDecimal(123.456)},
				{BareItem: BareItemBoolean(false)},
				{BareItem: BareItemBoolean(true)},
				{BareItem: BareItemDate(123456)},
				{BareItem: BareItemToken("token")},
				{BareItem: BareItemString("string")},
				{BareItem: BareItemDisplayString("display string")},
				{BareItem: BareItemByteSequence([]byte("test"))},
			},
		},
		want: `(123 123.456 ?0 ?1 @123456 token "string" %"display string" :dGVzdA==:)`,
	},
	{
		name: "with parameters",
		input: InnerList{
			Members: []Item{
				{BareItem: BareItemInteger(123)},
				{BareItem: BareItemDecimal(123.456)},
				{BareItem: BareItemBoolean(false)},
				{BareItem: BareItemBoolean(true)},
				{BareItem: BareItemDate(123456)},
				{BareItem: BareItemToken("token")},
				{BareItem: BareItemString("string")},
				{BareItem: BareItemDisplayString("display string")},
				{BareItem: BareItemByteSequence([]byte("test"))},
			},
			Parameters: params(
				"key1", BareItemToken("value1"),
				"key2", BareItemString("value2"),
			),
		},
		want: `(123 123.456 ?0 ?1 @123456 token "string" %"display string" :dGVzdA==:);key1=value1;key2="value2"`,
	},
	{
		name: "invalid value",
		input: InnerList{Members: []Item{
			{BareItem: BareItemToken(``)},
		}},
		wantError: ErrInvalidInnerList,
	},
	{
		name: "invalid parameters",
		input: InnerList{Members: []Item{
			{
				BareItem:   BareItemToken(`token`),
				Parameters: params("key", BareItemToken("")),
			},
		}},
		wantError: ErrInvalidInnerList,
	},
}

func Test_serializeInnerList(t *testing.T) {
	for _, testCase := range serializeInnerListTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := serializeInnerList([]byte("prefix: "), testCase.input)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("serializeInnerList() error %v, want %v", err, testCase.wantError)
			}

			if err != nil {
				return
			}

			want := "prefix: " + testCase.want

			if string(got) != want {
				t.Errorf("serializeInnerList() got = %q, want %q", got, want)
			}
		})
	}
}

func TestInnerList_AppendText(t *testing.T) {
	for _, testCase := range serializeInnerListTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := testCase.input.AppendText([]byte("prefix: "))

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("InnerList.AppendText() error %v, want %v", err, testCase.wantError)
			}

			if err != nil {
				return
			}

			want := "prefix: " + testCase.want

			if string(got) != want {
				t.Errorf("InnerList.AppendText() got = %q, want %q", got, want)
			}
		})
	}
}

type parseItemTestCase struct {
	name         string
	input        string
	want         Item
	wantRest     string
	wantError    error
	skipForParse bool
}

var parseItemTestCases []parseItemTestCase

type serializeItemTestCase struct {
	name      string
	input     Item
	want      string
	wantError error
}

var serializeItemTestCases []serializeItemTestCase

func init() {
	bareItemTestCasesByType := map[string][]parseBareItemTestCase{
		"integerOrDecimal": parseBareItemIntegerOrDecimalTestCases,
		"string":           parseBareItemStringTestCases,
		"token":            parseBareItemTokenTestCases,
		"byteSequence":     parseBareItemByteSequenceTestCases,
		"boolean":          parseBareItemBooleanTestCases,
		"date":             parseBareItemDateTestCases,
		"displayString":    parseBareItemDisplayStringTestCases,
	}

	parseBareItemTestCases = []parseBareItemTestCase{
		{
			name:      "empty input",
			wantError: ErrInvalidBareItem,
		},
	}

	inputParameters := `; key1=value1; key2="value2"; key3=%"value3"; key4=:dGVzdA==:; key5=?0; key6=?1; key7=@123456; key8=123; key9=123.456; key1=overridden`
	outputParameters := `;key1=overridden;key2="value2";key3=%"value3";key4=:dGVzdA==:;key5=?0;key6;key7=@123456;key8=123;key9=123.456`
	parameters := params(
		"key1", BareItemToken(`overridden`),
		"key2", BareItemString(`value2`),
		"key3", BareItemDisplayString(`value3`),
		"key4", BareItemByteSequence([]byte("test")),
		"key5", BareItemBoolean(false),
		"key6", BareItemBoolean(true),
		"key7", BareItemDate(123456),
		"key8", BareItemInteger(123),
		"key9", BareItemDecimal(123.456),
	)

	for type_, typeTestCases := range bareItemTestCasesByType {
		for _, testCase := range typeTestCases {
			if testCase.wantError != nil {
				continue
			}

			testCase.name = type_ + " - " + testCase.name
			parseBareItemTestCases = append(parseBareItemTestCases, testCase)

			parseItemTestCases = append(parseItemTestCases,
				parseItemTestCase{
					name:         testCase.name,
					input:        testCase.input,
					want:         Item{BareItem: testCase.want},
					wantRest:     testCase.wantRest,
					wantError:    testCase.wantError,
					skipForParse: testCase.skipForParse,
				},
			)

			if testCase.wantRest == "" {
				parseItemTestCases = append(parseItemTestCases,
					parseItemTestCase{
						name:         testCase.name + " - with parameters",
						input:        testCase.input + inputParameters,
						want:         Item{BareItem: testCase.want, Parameters: parameters},
						wantRest:     testCase.wantRest,
						wantError:    testCase.wantError,
						skipForParse: testCase.skipForParse,
					},
					parseItemTestCase{
						name:         testCase.name + " - with parameters separated with invalid space",
						input:        testCase.input + " " + inputParameters,
						want:         Item{BareItem: testCase.want},
						wantRest:     testCase.wantRest + " " + inputParameters,
						wantError:    testCase.wantError,
						skipForParse: true,
					})
			}
		}
	}

	serializeBareItemTestCasesByType := map[string][]serializeBareItemTestCase{
		"integer":       serializeBareItemIntegerTestCases,
		"decimal":       serializeBareItemDecimalTestCases,
		"string":        serializeBareItemStringTestCases,
		"token":         serializeBareItemTokenTestCases,
		"byteSequence":  serializeBareItemByteSequenceTestCases,
		"boolean":       serializeBareItemBooleanTestCases,
		"date":          serializeBareItemDateTestCases,
		"displayString": serializeBareItemDisplayStringTestCases,
	}

	for type_, typeTestCases := range serializeBareItemTestCasesByType {
		for _, testCase := range typeTestCases {
			testCase.name = type_ + " - " + testCase.name
			serializeBareItemTestCases = append(serializeBareItemTestCases, testCase)

			serializeItemTestCases = append(serializeItemTestCases,
				serializeItemTestCase{
					name:      testCase.name,
					input:     Item{BareItem: testCase.input},
					want:      testCase.want,
					wantError: testCase.wantError,
				},
				serializeItemTestCase{
					name:      testCase.name + " - with parameters",
					input:     Item{BareItem: testCase.input, Parameters: parameters},
					want:      testCase.want + outputParameters,
					wantError: testCase.wantError,
				},
				serializeItemTestCase{
					name:      testCase.name + " - with invalid parameters",
					input:     Item{BareItem: testCase.input, Parameters: params("", BareItemToken(""))},
					want:      testCase.want + outputParameters,
					wantError: ErrInvalidItem,
				},
			)
		}
	}
}

func Test_parseItem(t *testing.T) {
	for _, testCase := range parseItemTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			gotItem, gotRest, err := parseItem(testCase.input)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("parseItem() error %v, want %v", err, testCase.wantError)
			}

			if diff := cmp.Diff(testCase.want, gotItem, cmpOpts...); diff != "" {
				t.Errorf("parseItem() mismatch (-want, +got):\n%s", diff)
			}

			if gotRest != testCase.wantRest {
				t.Errorf("parseItem() gotRest = %q, want %q", gotRest, testCase.wantRest)
			}
		})
	}
}

func Test_serializeItem(t *testing.T) {
	for _, testCase := range serializeItemTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := serializeItem([]byte("prefix: "), testCase.input)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("serializeItem() error %v, want %v", err, testCase.wantError)
			}

			if err != nil {
				return
			}

			want := "prefix: " + testCase.want

			if string(got) != want {
				t.Errorf("serializeItem() got = %q, want %q", got, want)
			}
		})
	}
}

func TestItem_AppendText(t *testing.T) {
	for _, testCase := range serializeItemTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := testCase.input.AppendText([]byte("prefix: "))

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("Item.AppendText() error %v, want %v", err, testCase.wantError)
			}

			if err != nil {
				return
			}

			want := "prefix: " + testCase.want

			if string(got) != want {
				t.Errorf("Item.AppendText() got = %q, want %q", got, want)
			}
		})
	}
}

func Test_parseItemOrInnerList(t *testing.T) {
	type parseItemOrInnerListTestCase struct {
		name      string
		input     string
		want      ItemOrInnerList
		wantRest  string
		wantError error
	}

	var parseItemOrInnerListTestCases []parseItemOrInnerListTestCase

	for _, testCase := range parseItemTestCases {
		var want ItemOrInnerList

		if testCase.wantError == nil {
			want = ItemOrInnerListFrom(testCase.want)
		}

		parseItemOrInnerListTestCases = append(parseItemOrInnerListTestCases, parseItemOrInnerListTestCase{
			name:      "item - " + testCase.name,
			input:     testCase.input,
			want:      want,
			wantRest:  testCase.wantRest,
			wantError: testCase.wantError,
		})
	}

	for _, testCase := range parseInnerListTestCases {
		if testCase.input == "" {
			continue
		}

		var want ItemOrInnerList

		if testCase.wantError == nil {
			want = ItemOrInnerListFrom(testCase.want)
		}

		parseItemOrInnerListTestCases = append(parseItemOrInnerListTestCases, parseItemOrInnerListTestCase{
			name:      "inner list - " + testCase.name,
			input:     testCase.input,
			want:      want,
			wantRest:  testCase.wantRest,
			wantError: testCase.wantError,
		})
	}

	for _, testCase := range parseItemOrInnerListTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			gotItemOrInnerList, gotRest, err := parseItemOrInnerList(testCase.input)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("parseItemOrInnerList() error %v, want %v", err, testCase.wantError)
			}

			if diff := cmp.Diff(testCase.want, gotItemOrInnerList, cmpOpts...); diff != "" {
				t.Errorf("parseItemOrInnerList() mismatch (-want, +got):\n%s", diff)
			}

			if gotRest != testCase.wantRest {
				t.Errorf("parseItemOrInnerList() gotRest = %q, want %q", gotRest, testCase.wantRest)
			}
		})
	}
}

type parseListTestCase struct {
	name         string
	input        string
	want         List
	wantRest     string
	wantError    error
	skipForParse bool
}

var parseListTestCases = []parseListTestCase{
	{
		name: "empty input",
	},
	{
		name:  "list of items",
		input: `123 , 123.456 , ?0 , ?1 , @123456 , token , "string" ` + "\t" + `,` + "\t" + ` %"display string"`,
		want: List{
			Members: members(
				Item{BareItem: BareItemInteger(123)},
				Item{BareItem: BareItemDecimal(123.456)},
				Item{BareItem: BareItemBoolean(false)},
				Item{BareItem: BareItemBoolean(true)},
				Item{BareItem: BareItemDate(123456)},
				Item{BareItem: BareItemToken("token")},
				Item{BareItem: BareItemString("string")},
				Item{BareItem: BareItemDisplayString("display string")},
			),
		},
	},
	{
		name:  "list of inner lists",
		input: `( 123 123.456 ) , ( ?0  ?1 ) , ( @123456 token ) , ( "string" %"display string" )`,
		want: List{
			Members: members(
				InnerList{
					Members: []Item{
						{BareItem: BareItemInteger(123)},
						{BareItem: BareItemDecimal(123.456)},
					},
				},
				InnerList{
					Members: []Item{
						{BareItem: BareItemBoolean(false)},
						{BareItem: BareItemBoolean(true)},
					},
				},
				InnerList{
					Members: []Item{
						{BareItem: BareItemDate(123456)},
						{BareItem: BareItemToken("token")},
					},
				},
				InnerList{
					Members: []Item{
						{BareItem: BareItemString("string")},
						{BareItem: BareItemDisplayString("display string")},
					},
				},
			),
		},
	},
	{
		name:  "mixed list",
		input: `123,123.456,(?0 ?1),@123456,token,("string" %"display string")`,
		want: List{
			Members: members(
				Item{BareItem: BareItemInteger(123)},
				Item{BareItem: BareItemDecimal(123.456)},
				InnerList{
					Members: []Item{
						{BareItem: BareItemBoolean(false)},
						{BareItem: BareItemBoolean(true)},
					},
				},
				Item{BareItem: BareItemDate(123456)},
				Item{BareItem: BareItemToken("token")},
				InnerList{
					Members: []Item{
						{BareItem: BareItemString("string")},
						{BareItem: BareItemDisplayString("display string")},
					},
				},
			),
		},
	},
	{
		name:  "mixed list with parameters",
		input: `outer; a=1; b=2; a=3, (inner; a=10; b=20; a=30); a=100; b=200; a=300`,
		want: List{
			Members: members(
				Item{
					BareItem: BareItemToken("outer"),
					Parameters: params(
						"a", BareItemInteger(3),
						"b", BareItemInteger(2),
					),
				},
				InnerList{
					Members: []Item{
						{
							BareItem: BareItemToken("inner"),
							Parameters: params(
								"a", BareItemInteger(30),
								"b", BareItemInteger(20),
							),
						},
					},
					Parameters: params(
						"a", BareItemInteger(300),
						"b", BareItemInteger(200),
					),
				},
			),
		},
	},
	{
		name:  "empty inner list",
		input: `()`,
		want: List{
			Members: members(InnerList{}),
		},
	},
	{
		name:      "trailing comma",
		input:     `hello,`,
		wantError: ErrInvalidList,
	},
	{
		name:      "nested inner lists",
		input:     `(1 (2 3) 4)`,
		wantError: ErrInvalidList,
	},
	{
		name:      "tabs in inner list",
		input:     "(1\t2)",
		wantError: ErrInvalidList,
	},
	{
		name:      "comma in inner list",
		input:     "(1,2)",
		wantError: ErrInvalidList,
	},
	{
		name:      "invalid character after inner list item",
		input:     `(123a)`,
		wantError: ErrInvalidList,
	},
	{
		name:      "unclosed inner list",
		input:     `(123`,
		wantError: ErrInvalidList,
	},
	{
		name:      "unclosed inner list with parameters",
		input:     `(123; param=value`,
		wantError: ErrInvalidList,
	},
	{
		name:         "with leading spaces",
		input:        `   a, b`,
		wantError:    ErrInvalidList,
		skipForParse: true,
	},
	{
		name:  "with trailing spaces",
		input: `a, b   `,
		want: List{Members: members(
			Item{BareItem: BareItemToken("a")},
			Item{BareItem: BareItemToken("b")},
		)},
	},
}

func Test_parseList(t *testing.T) {
	for _, testCase := range parseListTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			gotList, gotRest, err := parseList(testCase.input)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("parseList() error %v, want %v", err, testCase.wantError)
			}

			if diff := cmp.Diff(testCase.want, gotList, cmpOpts...); diff != "" {
				t.Errorf("parseList() mismatch (-want, +got):\n%s", diff)
			}

			if gotRest != testCase.wantRest {
				t.Errorf("parseList() gotRest = %q, want %q", gotRest, testCase.wantRest)
			}
		})
	}
}

type serializeListTestCase struct {
	name      string
	input     List
	want      string
	wantError error
}

var serializeListTestCases = []serializeListTestCase{
	{
		name: "empty",
		want: ``,
	},
	{
		name: "single inner list",
		input: List{Members: members(
			InnerList{
				Members: []Item{
					{
						BareItem: BareItemToken(`token`),
						Parameters: params(
							"key1", BareItemToken("value1"),
							"key2", BareItemToken("value2"),
						),
					},
				},
				Parameters: params(
					"key3", BareItemToken("value3"),
					"key4", BareItemToken("value4"),
				),
			},
		)},
		want: `(token;key1=value1;key2=value2);key3=value3;key4=value4`,
	},
	{
		name: "single item",
		input: List{Members: members(
			Item{
				BareItem: BareItemToken(`token`),
				Parameters: params(
					"key1", BareItemToken("value1"),
					"key2", BareItemToken("value2"),
				),
			},
		)},
		want: `token;key1=value1;key2=value2`,
	},
	{
		name: "multiple members",
		input: List{Members: members(
			InnerList{
				Members: []Item{
					{
						BareItem: BareItemToken(`token`),
						Parameters: params(
							"key1", BareItemToken("value1"),
							"key2", BareItemToken("value2"),
						),
					},
				},
				Parameters: params(
					"key3", BareItemToken("value3"),
					"key4", BareItemToken("value4"),
				),
			},
			Item{
				BareItem: BareItemToken(`token`),
				Parameters: params(
					"key1", BareItemToken("value1"),
					"key2", BareItemToken("value2"),
				),
			},
		)},
		want: `(token;key1=value1;key2=value2);key3=value3;key4=value4, token;key1=value1;key2=value2`,
	},
	{
		name: "invalid inner list",
		input: List{Members: members(
			InnerList{Members: []Item{
				{BareItem: BareItemToken("")},
			}},
		)},
		wantError: ErrInvalidList,
	},
	{
		name: "invalid item",
		input: List{Members: members(
			Item{BareItem: BareItemToken("")},
		)},
		wantError: ErrInvalidList,
	},
}

func Test_serializeList(t *testing.T) {
	for _, testCase := range serializeListTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := serializeList([]byte("prefix: "), testCase.input)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("serializeList() error %v, want %v", err, testCase.wantError)
			}

			if err != nil {
				return
			}

			want := "prefix: " + testCase.want

			if string(got) != want {
				t.Errorf("serializeList() got = %q, want %q", got, want)
			}
		})
	}
}

func TestList_AppendText(t *testing.T) {
	for _, testCase := range serializeListTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := testCase.input.AppendText([]byte("prefix: "))

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("List.AppendText() error %v, want %v", err, testCase.wantError)
			}

			if err != nil {
				return
			}

			want := "prefix: " + testCase.want

			if string(got) != want {
				t.Errorf("List.AppendText() got = %q, want %q", got, want)
			}
		})
	}
}

type parseParametersTestCase struct {
	name      string
	input     string
	want      Parameters
	wantRest  string
	wantError error
}

var parseParametersTestCases = []parseParametersTestCase{
	{
		name: "empty input",
	},
	{
		name:      "empty name",
		input:     `;`,
		wantError: ErrInvalidKey,
	},
	{
		name:      "empty names",
		input:     `;;`,
		wantError: ErrInvalidKey,
	},
	{
		name:  "single",
		input: `; key=value`,
		want:  params("key", BareItemToken(`value`)),
	},
	{
		name:  "multiple",
		input: `; key1=value1; key2="value2"; key3=%"value3"; key4=:dGVzdA==:; key5=?0; key6=?1; key7=@123456; key8=123; key9=123.456; key1=overridden`,
		want: params(
			"key1", BareItemToken(`overridden`),
			"key2", BareItemString(`value2`),
			"key3", BareItemDisplayString(`value3`),
			"key4", BareItemByteSequence([]byte("test")),
			"key5", BareItemBoolean(false),
			"key6", BareItemBoolean(true),
			"key7", BareItemDate(123456),
			"key8", BareItemInteger(123),
			"key9", BareItemDecimal(123.456),
		),
	},
	{
		name:  "complex key",
		input: "; *key_with-many.chars_*=1",
		want:  params("*key_with-many.chars_*", BareItemInteger(1)),
	},
	{
		name:  "boolean parameter",
		input: "; bool; non-bool=1",
		want: params(
			"bool", BareItemBoolean(true),
			"non-bool", BareItemInteger(1),
		),
	},
	{
		name:      "inner list",
		input:     `; key=(inner list)`,
		wantError: ErrInvalidParameters,
	},
}

func Test_parseParameters(t *testing.T) {
	for _, testCase := range parseParametersTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			gotBareItem, gotRest, err := parseParameters(testCase.input)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("parseParameters() error %v, want %v", err, testCase.wantError)
			}

			if diff := cmp.Diff(testCase.want, gotBareItem, cmpOpts...); diff != "" {
				t.Errorf("parseParameters() mismatch (-want, +got):\n%s", diff)
			}

			if gotRest != testCase.wantRest {
				t.Errorf("parseParameters() gotRest = %q, want %q", gotRest, testCase.wantRest)
			}
		})
	}
}

type serializeParametersTestCase struct {
	name      string
	input     Parameters
	want      string
	wantError error
}

var serializeParametersTestCases = []serializeParametersTestCase{
	{
		name: "empty",
	},
	{
		name:  "single parameter",
		input: params("key1", BareItemToken(`value1`)),
		want:  `;key1=value1`,
	},
	{
		name: "multiple parameter",
		input: params(
			"key1", BareItemToken(`value1`),
			"key2", BareItemString(`value2`),
			"key3", BareItemDisplayString(`value3`),
			"key4", BareItemByteSequence([]byte("test")),
			"key5", BareItemBoolean(false),
			"key6", BareItemBoolean(true),
			"key7", BareItemDate(123456),
			"key8", BareItemInteger(123),
			"key9", BareItemDecimal(123.456),
		),
		want: `;key1=value1;key2="value2";key3=%"value3";key4=:dGVzdA==:;key5=?0;key6;key7=@123456;key8=123;key9=123.456`,
	},
	{
		name:      "invalid key",
		input:     params("", BareItemToken(`value1`)),
		wantError: ErrInvalidParameters,
	},
	{
		name:      "invalid value",
		input:     params("key1", BareItemToken(``)),
		wantError: ErrInvalidParameters,
	},
}

func Test_serializeParameters(t *testing.T) {
	for _, testCase := range serializeParametersTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := serializeParameters([]byte("prefix: "), testCase.input)

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("serializeParameters() error %v, want %v", err, testCase.wantError)
			}

			if err != nil {
				return
			}

			gotStr := string(got)
			want := "prefix: " + testCase.want

			if gotStr != want {
				t.Errorf("serializeParameters() got = %q, want %q", gotStr, want)
			}
		})
	}
}

func TestParameters_AppendText(t *testing.T) {
	for _, testCase := range serializeParametersTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := testCase.input.AppendText([]byte("prefix: "))

			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("Parameters.AppendText() error %v, want %v", err, testCase.wantError)
			}

			if err != nil {
				return
			}

			gotStr := string(got)
			want := "prefix: " + testCase.want

			if gotStr != want {
				t.Errorf("Parameters.AppendText() got = %q, want %q", gotStr, want)
			}
		})
	}
}
