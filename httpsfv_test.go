package httpsfv

import (
	"errors"
	"fmt"
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
			value = ItemOrInnerList{Type: ItemOrInnerListTypeInnerList, InnerList: v}
		case Item:
			value = ItemOrInnerList{Type: ItemOrInnerListTypeItem, Item: v}
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
			ms = append(ms, ItemOrInnerList{Type: ItemOrInnerListTypeInnerList, InnerList: v})
		case Item:
			ms = append(ms, ItemOrInnerList{Type: ItemOrInnerListTypeItem, Item: v})
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
					Item{BareItem: BareItem{Type: BareItemTypeToken, Token: "hello"}},
					Item{BareItem: BareItem{Type: BareItemTypeString, String: "world"}},
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
					Item{BareItem: BareItem{Type: BareItemTypeToken, Token: "hello"}},
					Item{BareItem: BareItem{Type: BareItemTypeString, String: "world"}},
					Item{BareItem: BareItem{Type: BareItemTypeToken, Token: "how"}},
					Item{BareItem: BareItem{Type: BareItemTypeString, String: "are you?"}},
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

var parseBareItemIntegerOrDecimalTestCases = []parseBareItemTestCase{
	{
		name:         "empty input",
		wantError:    ErrInvalidIntegerOrDecimal,
		skipForParse: true,
	},
	{
		name:  "decimal",
		input: "123.456",
		want:  BareItem{Type: BareItemTypeDecimal, Decimal: 123.456},
	},
	{
		name:  "negative decimal",
		input: "-123.456",
		want:  BareItem{Type: BareItemTypeDecimal, Decimal: -123.456},
	},
	{
		name:      "decimal with plus sign",
		input:     "+123.456",
		wantError: ErrInvalidIntegerOrDecimal,
	},
	{
		name:  "decimal with exactly 12 characters before dot",
		input: "123456789012.34",
		want:  BareItem{Type: BareItemTypeDecimal, Decimal: 123456789012.34},
	},
	{
		name:      "decimal with more than 12 characters before dot",
		input:     "1234567890123.4",
		wantError: ErrInvalidIntegerOrDecimal,
	},
	{
		name:  "decimal with exactly 16 characters",
		input: "123456789012.345",
		want:  BareItem{Type: BareItemTypeDecimal, Decimal: 123456789012.345},
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
		want:         BareItem{Type: BareItemTypeDecimal, Decimal: 123.456},
		wantRest:     "   ",
		skipForParse: true,
	},
	{
		name:         "trailing data after decimal",
		input:        "123.456abc",
		want:         BareItem{Type: BareItemTypeDecimal, Decimal: 123.456},
		wantRest:     "abc",
		skipForParse: true,
	},

	{
		name:  "integer",
		input: "123456",
		want:  BareItem{Type: BareItemTypeInteger, Integer: 123_456},
	},
	{
		name:  "negative integer",
		input: "-123456",
		want:  BareItem{Type: BareItemTypeInteger, Integer: -123_456},
	},
	{
		name:      "integer with plus sign",
		input:     "+123456",
		wantError: ErrInvalidIntegerOrDecimal,
	},
	{
		name:  "integer with exactly 15 characters",
		input: "123456789012345",
		want:  BareItem{Type: BareItemTypeInteger, Integer: 123_456_789_012_345},
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
		want:         BareItem{Type: BareItemTypeInteger, Integer: 123},
		wantRest:     "   ",
		skipForParse: true,
	},
	{
		name:         "trailing data after integer",
		input:        "123abc",
		want:         BareItem{Type: BareItemTypeInteger, Integer: 123},
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

var parseBareItemStringTestCases = []parseBareItemTestCase{
	{
		name:         "empty input",
		wantError:    ErrInvalidString,
		skipForParse: true,
	},
	{
		name:  "empty",
		input: `""`,
		want:  BareItem{Type: BareItemTypeString, String: ``},
	},
	{
		name:  "simple",
		input: `"hello world"`,
		want:  BareItem{Type: BareItemTypeString, String: `hello world`},
	},
	{
		name:  "with escaped characters",
		input: `"with \" and \\"`,
		want:  BareItem{Type: BareItemTypeString, String: `with " and \`},
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
		want:         BareItem{Type: BareItemTypeString, String: `hello world`},
		wantRest:     `   `,
		skipForParse: true,
	},
	{
		name:         "with trailing data",
		input:        `"hello world"abc`,
		want:         BareItem{Type: BareItemTypeString, String: `hello world`},
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

var parseBareItemTokenTestCases = []parseBareItemTestCase{
	{
		name:         "empty input",
		wantError:    ErrInvalidToken,
		skipForParse: true,
	},
	{
		name:  "simple",
		input: "hello",
		want:  BareItem{Type: BareItemTypeToken, Token: "hello"},
	},
	{
		name:  "starting with upper case character",
		input: "Hello",
		want:  BareItem{Type: BareItemTypeToken, Token: "Hello"},
	},
	{
		name:  "starting with *",
		input: "*hello",
		want:  BareItem{Type: BareItemTypeToken, Token: "*hello"},
	},
	{
		name:  "with complex characters",
		input: "h!#$%&'+-.^_`|~:/",
		want:  BareItem{Type: BareItemTypeToken, Token: "h!#$%&'+-.^_`|~:/"},
	},
	{
		name:  "with invalid first character",
		input: "h!#$%&'+-.^_`|~:/",
		want:  BareItem{Type: BareItemTypeToken, Token: "h!#$%&'+-.^_`|~:/"},
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
		want:         BareItem{Type: BareItemTypeToken, Token: "hello"},
		wantRest:     "   ",
		skipForParse: true,
	},
	{
		name:         "with trailing data",
		input:        "hello(x)",
		want:         BareItem{Type: BareItemTypeToken, Token: "hello"},
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

var parseBareItemByteSequenceTestCases = []parseBareItemTestCase{
	{
		name:         "empty input",
		wantError:    ErrInvalidByteSequence,
		skipForParse: true,
	},
	{
		name:  "valid",
		input: ":dGVzdA==:",
		want:  BareItem{Type: BareItemTypeByteSequence, ByteSequence: []byte("test")},
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
		want:  BareItem{Type: BareItemTypeByteSequence, ByteSequence: []byte("test")},
	},
	{
		name:  "without data",
		input: "::",
		want:  BareItem{Type: BareItemTypeByteSequence, ByteSequence: []byte{}},
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
		want:         BareItem{Type: BareItemTypeByteSequence, ByteSequence: []byte("test")},
		wantRest:     "   ",
		skipForParse: true,
	},
	{
		name:         "with trailing data",
		input:        ":dGVzdA==::x:",
		want:         BareItem{Type: BareItemTypeByteSequence, ByteSequence: []byte("test")},
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

var parseBareItemBooleanTestCases = []parseBareItemTestCase{
	{
		name:         "empty input",
		wantError:    ErrInvalidBoolean,
		skipForParse: true,
	},
	{
		name:  "boolean true",
		input: "?1",
		want:  BareItem{Type: BareItemTypeBoolean, Boolean: true},
	},
	{
		name:  "boolean false",
		input: "?0",
		want:  BareItem{Type: BareItemTypeBoolean, Boolean: false},
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
		want:         BareItem{Type: BareItemTypeBoolean, Boolean: false},
		wantRest:     "   ",
		skipForParse: true,
	},
	{
		name:         "with trailing data",
		input:        "?01234",
		want:         BareItem{Type: BareItemTypeBoolean, Boolean: false},
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

var parseBareItemDateTestCases = []parseBareItemTestCase{
	{
		name:         "empty input",
		wantError:    ErrInvalidDate,
		skipForParse: true,
	},
	{
		name:  "positive",
		input: "@123456",
		want:  BareItem{Type: BareItemTypeDate, Date: 123456},
	},
	{
		name:  "negative date",
		input: "@-123456",
		want:  BareItem{Type: BareItemTypeDate, Date: -123456},
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
		want:         BareItem{Type: BareItemTypeDate, Date: 123456},
		wantRest:     "   ",
		skipForParse: true,
	},
	{
		name:         "with trailing data",
		input:        "@123456abc",
		want:         BareItem{Type: BareItemTypeDate, Date: 123456},
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

var parseBareItemDisplayStringTestCases = []parseBareItemTestCase{
	{
		name:         "empty input",
		wantError:    ErrInvalidDisplayString,
		skipForParse: true,
	},
	{
		name:  "valid",
		input: `%"hello world"`,
		want:  BareItem{Type: BareItemTypeDisplayString, DisplayString: `hello world`},
	},
	{
		name:  "with hex",
		input: `%"%c3%bcsers"`,
		want:  BareItem{Type: BareItemTypeDisplayString, DisplayString: `üsers`},
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
		want:         BareItem{Type: BareItemTypeDisplayString, DisplayString: `hello world`},
		wantRest:     "   ",
		skipForParse: true,
	},
	{
		name:         "with trailing data",
		input:        `%"hello world"%"test"`,
		want:         BareItem{Type: BareItemTypeDisplayString, DisplayString: `hello world`},
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
			"a", Item{BareItem: BareItem{Type: BareItemTypeInteger, Integer: 123}},
			"b", Item{BareItem: BareItem{Type: BareItemTypeDecimal, Decimal: 123.456}},
			"c", Item{BareItem: BareItem{Type: BareItemTypeBoolean, Boolean: false}},
			"d", Item{BareItem: BareItem{Type: BareItemTypeBoolean, Boolean: true}},
			"e", Item{BareItem: BareItem{Type: BareItemTypeDate, Date: 123456}},
			"f", Item{BareItem: BareItem{Type: BareItemTypeToken, Token: "token"}},
			"g", Item{BareItem: BareItem{Type: BareItemTypeString, String: "string"}},
			"h", Item{BareItem: BareItem{Type: BareItemTypeDisplayString, DisplayString: "display string"}},
		),
	},
	{
		name:  "mixed dictionary",
		input: `a=123,b=123.456,c=(?0 ?1),d=@123456,e=token,f=("string" %"display string")`,
		want: dict(
			"a", Item{BareItem: BareItem{Type: BareItemTypeInteger, Integer: 123}},
			"b", Item{BareItem: BareItem{Type: BareItemTypeDecimal, Decimal: 123.456}},
			"c", InnerList{
				Members: []Item{
					{BareItem: BareItem{Type: BareItemTypeBoolean, Boolean: false}},
					{BareItem: BareItem{Type: BareItemTypeBoolean, Boolean: true}},
				},
			},
			"d", Item{BareItem: BareItem{Type: BareItemTypeDate, Date: 123456}},
			"e", Item{BareItem: BareItem{Type: BareItemTypeToken, Token: "token"}},
			"f", InnerList{
				Members: []Item{
					{BareItem: BareItem{Type: BareItemTypeString, String: "string"}},
					{BareItem: BareItem{Type: BareItemTypeDisplayString, DisplayString: "display string"}},
				},
			},
		),
	},
	{
		name:  "boolean without value",
		input: `a`,
		want: dict(
			"a", Item{BareItem: BareItem{Type: BareItemTypeBoolean, Boolean: true}},
		),
	},
	{
		name:  "boolean without value but with parameters",
		input: `a; param1=value1;param2=value2; param1=value3`,
		want: dict(
			"a", Item{
				BareItem: BareItem{Type: BareItemTypeBoolean, Boolean: true},
				Parameters: params(
					"param1", BareItem{Type: BareItemTypeToken, Token: "value3"},
					"param2", BareItem{Type: BareItemTypeToken, Token: "value2"},
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
				{BareItem: BareItem{Type: BareItemTypeInteger, Integer: 123}},
				{BareItem: BareItem{Type: BareItemTypeDecimal, Decimal: 123.456}},
				{BareItem: BareItem{Type: BareItemTypeBoolean, Boolean: false}},
				{BareItem: BareItem{Type: BareItemTypeBoolean, Boolean: true}},
				{BareItem: BareItem{Type: BareItemTypeDate, Date: 123456}},
				{BareItem: BareItem{Type: BareItemTypeToken, Token: "token"}},
				{BareItem: BareItem{Type: BareItemTypeString, String: "string"}},
				{BareItem: BareItem{Type: BareItemTypeDisplayString, DisplayString: "display string"}},
				{BareItem: BareItem{Type: BareItemTypeByteSequence, ByteSequence: []byte("test")}},
			},
		},
	},
	{
		name:  "all items with parameters",
		input: `( 123; a=1 123.456; b=2 ?0; c=3 ?1; d=4 @123456; e=5 token; f=6 "string"; g=7 %"display string"; h=8 :dGVzdA==:; i=9 )`,
		want: InnerList{
			Members: []Item{
				{
					BareItem:   BareItem{Type: BareItemTypeInteger, Integer: 123},
					Parameters: params("a", BareItem{Type: BareItemTypeInteger, Integer: 1}),
				},
				{
					BareItem:   BareItem{Type: BareItemTypeDecimal, Decimal: 123.456},
					Parameters: params("b", BareItem{Type: BareItemTypeInteger, Integer: 2}),
				},
				{
					BareItem:   BareItem{Type: BareItemTypeBoolean, Boolean: false},
					Parameters: params("c", BareItem{Type: BareItemTypeInteger, Integer: 3}),
				},
				{
					BareItem:   BareItem{Type: BareItemTypeBoolean, Boolean: true},
					Parameters: params("d", BareItem{Type: BareItemTypeInteger, Integer: 4}),
				},
				{
					BareItem:   BareItem{Type: BareItemTypeDate, Date: 123456},
					Parameters: params("e", BareItem{Type: BareItemTypeInteger, Integer: 5}),
				},
				{
					BareItem:   BareItem{Type: BareItemTypeToken, Token: "token"},
					Parameters: params("f", BareItem{Type: BareItemTypeInteger, Integer: 6}),
				},
				{
					BareItem:   BareItem{Type: BareItemTypeString, String: "string"},
					Parameters: params("g", BareItem{Type: BareItemTypeInteger, Integer: 7}),
				},
				{
					BareItem:   BareItem{Type: BareItemTypeDisplayString, DisplayString: "display string"},
					Parameters: params("h", BareItem{Type: BareItemTypeInteger, Integer: 8}),
				},
				{
					BareItem:   BareItem{Type: BareItemTypeByteSequence, ByteSequence: []byte("test")},
					Parameters: params("i", BareItem{Type: BareItemTypeInteger, Integer: 9}),
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

type parseItemTestCase struct {
	name         string
	input        string
	want         Item
	wantRest     string
	wantError    error
	skipForParse bool
}

var parseItemTestCases []parseItemTestCase

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
	wantParameters := params(
		"key1", BareItem{Type: BareItemTypeToken, Token: `overridden`},
		"key2", BareItem{Type: BareItemTypeString, String: `value2`},
		"key3", BareItem{Type: BareItemTypeDisplayString, DisplayString: `value3`},
		"key4", BareItem{Type: BareItemTypeByteSequence, ByteSequence: []byte("test")},
		"key5", BareItem{Type: BareItemTypeBoolean, Boolean: false},
		"key6", BareItem{Type: BareItemTypeBoolean, Boolean: true},
		"key7", BareItem{Type: BareItemTypeDate, Date: 123456},
		"key8", BareItem{Type: BareItemTypeInteger, Integer: 123},
		"key9", BareItem{Type: BareItemTypeDecimal, Decimal: 123.456},
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
						want:         Item{BareItem: testCase.want, Parameters: wantParameters},
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
			want = ItemOrInnerList{Type: ItemOrInnerListTypeItem, Item: testCase.want}
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
			want = ItemOrInnerList{Type: ItemOrInnerListTypeInnerList, InnerList: testCase.want}
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
				Item{BareItem: BareItem{Type: BareItemTypeInteger, Integer: 123}},
				Item{BareItem: BareItem{Type: BareItemTypeDecimal, Decimal: 123.456}},
				Item{BareItem: BareItem{Type: BareItemTypeBoolean, Boolean: false}},
				Item{BareItem: BareItem{Type: BareItemTypeBoolean, Boolean: true}},
				Item{BareItem: BareItem{Type: BareItemTypeDate, Date: 123456}},
				Item{BareItem: BareItem{Type: BareItemTypeToken, Token: "token"}},
				Item{BareItem: BareItem{Type: BareItemTypeString, String: "string"}},
				Item{BareItem: BareItem{Type: BareItemTypeDisplayString, DisplayString: "display string"}},
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
						{BareItem: BareItem{Type: BareItemTypeInteger, Integer: 123}},
						{BareItem: BareItem{Type: BareItemTypeDecimal, Decimal: 123.456}},
					},
				},
				InnerList{
					Members: []Item{
						{BareItem: BareItem{Type: BareItemTypeBoolean, Boolean: false}},
						{BareItem: BareItem{Type: BareItemTypeBoolean, Boolean: true}},
					},
				},
				InnerList{
					Members: []Item{
						{BareItem: BareItem{Type: BareItemTypeDate, Date: 123456}},
						{BareItem: BareItem{Type: BareItemTypeToken, Token: "token"}},
					},
				},
				InnerList{
					Members: []Item{
						{BareItem: BareItem{Type: BareItemTypeString, String: "string"}},
						{BareItem: BareItem{Type: BareItemTypeDisplayString, DisplayString: "display string"}},
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
				Item{BareItem: BareItem{Type: BareItemTypeInteger, Integer: 123}},
				Item{BareItem: BareItem{Type: BareItemTypeDecimal, Decimal: 123.456}},
				InnerList{
					Members: []Item{
						{BareItem: BareItem{Type: BareItemTypeBoolean, Boolean: false}},
						{BareItem: BareItem{Type: BareItemTypeBoolean, Boolean: true}},
					},
				},
				Item{BareItem: BareItem{Type: BareItemTypeDate, Date: 123456}},
				Item{BareItem: BareItem{Type: BareItemTypeToken, Token: "token"}},
				InnerList{
					Members: []Item{
						{BareItem: BareItem{Type: BareItemTypeString, String: "string"}},
						{BareItem: BareItem{Type: BareItemTypeDisplayString, DisplayString: "display string"}},
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
					BareItem: BareItem{Type: BareItemTypeToken, Token: "outer"},
					Parameters: params(
						"a", BareItem{Type: BareItemTypeInteger, Integer: 3},
						"b", BareItem{Type: BareItemTypeInteger, Integer: 2},
					),
				},
				InnerList{
					Members: []Item{
						{
							BareItem: BareItem{Type: BareItemTypeToken, Token: "inner"},
							Parameters: params(
								"a", BareItem{Type: BareItemTypeInteger, Integer: 30},
								"b", BareItem{Type: BareItemTypeInteger, Integer: 20},
							),
						},
					},
					Parameters: params(
						"a", BareItem{Type: BareItemTypeInteger, Integer: 300},
						"b", BareItem{Type: BareItemTypeInteger, Integer: 200},
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
			Item{BareItem: BareItem{Type: BareItemTypeToken, Token: "a"}},
			Item{BareItem: BareItem{Type: BareItemTypeToken, Token: "b"}},
		)},
	},
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
		want:  params("key", BareItem{Type: BareItemTypeToken, Token: `value`}),
	},
	{
		name:  "multiple",
		input: `; key1=value1; key2="value2"; key3=%"value3"; key4=:dGVzdA==:; key5=?0; key6=?1; key7=@123456; key8=123; key9=123.456; key1=overridden`,
		want: params(
			"key1", BareItem{Type: BareItemTypeToken, Token: `overridden`},
			"key2", BareItem{Type: BareItemTypeString, String: `value2`},
			"key3", BareItem{Type: BareItemTypeDisplayString, DisplayString: `value3`},
			"key4", BareItem{Type: BareItemTypeByteSequence, ByteSequence: []byte("test")},
			"key5", BareItem{Type: BareItemTypeBoolean, Boolean: false},
			"key6", BareItem{Type: BareItemTypeBoolean, Boolean: true},
			"key7", BareItem{Type: BareItemTypeDate, Date: 123456},
			"key8", BareItem{Type: BareItemTypeInteger, Integer: 123},
			"key9", BareItem{Type: BareItemTypeDecimal, Decimal: 123.456},
		),
	},
	{
		name:  "complex key",
		input: "; *key_with-many.chars_*=1",
		want:  params("*key_with-many.chars_*", BareItem{Type: BareItemTypeInteger, Integer: 1}),
	},
	{
		name:  "boolean parameter",
		input: "; bool; non-bool=1",
		want: params(
			"bool", BareItem{Type: BareItemTypeBoolean, Boolean: true},
			"non-bool", BareItem{Type: BareItemTypeInteger, Integer: 1},
		),
	},
	{
		name:      "inner list",
		input:     `; key=(inner list)`,
		wantError: ErrInvalidParameters,
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
