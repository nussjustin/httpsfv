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

func TestParse(t *testing.T) {
	t.Run("Dictionary", func(t *testing.T) {
		testCases := []struct {
			name      string
			input     string
			want      Dictionary
			wantError error
		}{
			{
				name: "empty",
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
	})

	t.Run("Item", func(t *testing.T) {
		testCases := []struct {
			name      string
			input     string
			want      Item
			wantError error
		}{
			{
				name: "empty",
			},
			{
				name:      "non-ascii",
				input:     "hêllo",
				wantError: ErrNonAsciiInput,
			},
			{
				name:      "only spaces",
				input:     " ",
				wantError: ErrInvalidBareItem,
			},

			{
				name:  "decimal",
				input: "123.456",
				want:  Item{BareItem: BareItem{Type: BareItemTypeDecimal, Decimal: 123.456}},
			},
			{
				name:  "negative decimal",
				input: "-123.456",
				want:  Item{BareItem: BareItem{Type: BareItemTypeDecimal, Decimal: -123.456}},
			},
			{
				name:  "decimal with exactly 12 characters before dot",
				input: "123456789012.34",
				want:  Item{BareItem: BareItem{Type: BareItemTypeDecimal, Decimal: 123456789012.34}},
			},
			{
				name:      "decimal with more than 12 characters before dot",
				input:     "1234567890123.4",
				wantError: ErrInvalidIntegerOrDecimal,
			},
			{
				name:  "decimal with exactly 16 characters",
				input: "123456789012.345",
				want:  Item{BareItem: BareItem{Type: BareItemTypeDecimal, Decimal: 123456789012.345}},
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
				name:  "decimal with parameters",
				input: "123.456; param1=1.2;param2=2.3; param1=3.4",
				want: Item{
					BareItem:   BareItem{Type: BareItemTypeDecimal, Decimal: 123.456},
					Parameters: params("param1", BareItem{Type: BareItemTypeDecimal, Decimal: 3.4}, "param2", BareItem{Type: BareItemTypeDecimal, Decimal: 2.3}),
				},
			},

			{
				name:  "integer",
				input: "123456",
				want:  Item{BareItem: BareItem{Type: BareItemTypeInteger, Integer: 123_456}},
			},
			{
				name:  "negative integer",
				input: "-123456",
				want:  Item{BareItem: BareItem{Type: BareItemTypeInteger, Integer: -123_456}},
			},
			{
				name:  "integer with exactly 15 characters",
				input: "123456789012345",
				want:  Item{BareItem: BareItem{Type: BareItemTypeInteger, Integer: 123_456_789_012_345}},
			},
			{
				name:      "integer with more than 15 characters",
				input:     "1234567890123456",
				wantError: ErrInvalidIntegerOrDecimal,
			},
			{
				name:  "integer with parameters",
				input: "123; param1=1;param2=2; param1=3",
				want: Item{
					BareItem:   BareItem{Type: BareItemTypeInteger, Integer: 123},
					Parameters: params("param1", BareItem{Type: BareItemTypeInteger, Integer: 3}, "param2", BareItem{Type: BareItemTypeInteger, Integer: 2}),
				},
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

			{
				name:  "string",
				input: `"hello world"`,
				want: Item{
					BareItem: BareItem{Type: BareItemTypeString, String: `hello world`},
				},
			},
			{
				name:  "string without content",
				input: `""`,
				want: Item{
					BareItem: BareItem{Type: BareItemTypeString, String: ``},
				},
			},
			{
				name:  "string with escaped characters",
				input: `"with \" and \\"`,
				want: Item{
					BareItem: BareItem{Type: BareItemTypeString, String: `with " and \`},
				},
			},
			{
				name:      "string with invalid escape",
				input:     `"\a"`,
				wantError: ErrInvalidString,
			},
			{
				name:      "string with invalid character",
				input:     `"hello` + string([]byte{0}) + `"`,
				wantError: ErrInvalidString,
			},
			{
				name:      "string without ending quote",
				input:     `"hello world`,
				wantError: ErrInvalidString,
			},
			{
				name:  "string with parameters",
				input: `"hello world"; param1="value 1";param2="value 2"; param1="value 3"`,
				want: Item{
					BareItem: BareItem{Type: BareItemTypeString, String: `hello world`},
					Parameters: params(
						"param1", BareItem{Type: BareItemTypeString, String: `value 3`},
						"param2", BareItem{Type: BareItemTypeString, String: `value 2`},
					),
				},
			},

			{
				name:  "token",
				input: "hello",
				want: Item{
					BareItem: BareItem{Type: BareItemTypeToken, Token: "hello"},
				},
			},
			{
				name:  "token starting with upper case character",
				input: "Hello",
				want: Item{
					BareItem: BareItem{Type: BareItemTypeToken, Token: "Hello"},
				},
			},
			{
				name:  "token starting with *",
				input: "*hello",
				want: Item{
					BareItem: BareItem{Type: BareItemTypeToken, Token: "*hello"},
				},
			},
			{
				name:  "token with complex characters",
				input: "h!#$%&'+-.^_`|~:/",
				want: Item{
					BareItem: BareItem{Type: BareItemTypeToken, Token: "h!#$%&'+-.^_`|~:/"},
				},
			},
			{
				name:  "token with invalid first character",
				input: "h!#$%&'+-.^_`|~:/",
				want: Item{
					BareItem: BareItem{Type: BareItemTypeToken, Token: "h!#$%&'+-.^_`|~:/"},
				},
			},
			{
				name:  "token with parameters",
				input: "hello; param1=value1;param2=value2; param1=value3",
				want: Item{
					BareItem: BareItem{Type: BareItemTypeToken, Token: "hello"},
					Parameters: params(
						"param1", BareItem{Type: BareItemTypeToken, Token: "value3"},
						"param2", BareItem{Type: BareItemTypeToken, Token: "value2"},
					),
				},
			},

			{
				name:  "byte sequence",
				input: ":dGVzdA==:",
				want: Item{
					BareItem: BareItem{Type: BareItemTypeByteSequence, ByteSequence: []byte("test")},
				},
			},
			{
				name:  "byte sequence without padding",
				input: ":dGVzdA:",
				want: Item{
					BareItem: BareItem{Type: BareItemTypeByteSequence, ByteSequence: []byte("test")},
				},
			},
			{
				name:      "byte sequence without trailing :",
				input:     ":dGVzdA==",
				wantError: ErrInvalidByteSequence,
			},
			{
				name:  "byte sequence without data",
				input: "::",
				want: Item{
					BareItem: BareItem{Type: BareItemTypeByteSequence, ByteSequence: []byte{}},
				},
			},
			{
				name:      "byte sequence with invalid characters",
				input:     ":dGVzdA== :",
				wantError: ErrInvalidByteSequence,
			},
			{
				name:      "byte sequence with invalid base64",
				input:     ":==dGVzdA==:",
				wantError: ErrInvalidByteSequence,
			},
			{
				name:  "byte sequence with parameters",
				input: ":dGVzdA==:; param1=:dmFsdWUx:;param2=:dmFsdWUy:; param1=:dmFsdWUz:",
				want: Item{
					BareItem: BareItem{Type: BareItemTypeByteSequence, ByteSequence: []byte("test")},
					Parameters: params(
						"param1", BareItem{Type: BareItemTypeByteSequence, ByteSequence: []byte("value3")},
						"param2", BareItem{Type: BareItemTypeByteSequence, ByteSequence: []byte("value2")},
					),
				},
			},

			{
				name:  "boolean true",
				input: "?1",
				want:  Item{BareItem: BareItem{Type: BareItemTypeBoolean, Boolean: true}},
			},
			{
				name:  "boolean false",
				input: "?0",
				want:  Item{BareItem: BareItem{Type: BareItemTypeBoolean, Boolean: false}},
			},
			{
				name:      "invalid boolean number",
				input:     "?2",
				wantError: ErrInvalidBoolean,
			},
			{
				name:      "more digits after boolean",
				input:     "?01",
				wantError: ErrTrailingData,
			},
			{
				name:  "boolean with parameters",
				input: "?1; param1=?0;param2=?0; param1=?1",
				want: Item{
					BareItem:   BareItem{Type: BareItemTypeBoolean, Boolean: true},
					Parameters: params("param1", BareItem{Type: BareItemTypeBoolean, Boolean: true}, "param2", BareItem{Type: BareItemTypeBoolean, Boolean: false}),
				},
			},

			{
				name:  "date",
				input: "@123456",
				want:  Item{BareItem: BareItem{Type: BareItemTypeDate, Date: 123456}},
			},
			{
				name:  "negative date",
				input: "@-123456",
				want:  Item{BareItem: BareItem{Type: BareItemTypeDate, Date: -123456}},
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
				name:      "decimal date",
				input:     "@123.456",
				wantError: ErrInvalidDate,
			},
			{
				name:      "date with more than 16 characters",
				input:     "@12345678901234567",
				wantError: ErrInvalidDate,
			},
			{
				name:  "date with parameters",
				input: "@123; param1=@1;param2=@2; param1=@3",
				want: Item{
					BareItem:   BareItem{Type: BareItemTypeDate, Date: 123},
					Parameters: params("param1", BareItem{Type: BareItemTypeDate, Date: 3}, "param2", BareItem{Type: BareItemTypeDate, Date: 2}),
				},
			},

			{
				name:  "display string",
				input: `%"hello world"`,
				want: Item{
					BareItem: BareItem{Type: BareItemTypeDisplayString, DisplayString: `hello world`},
				},
			},
			{
				name:  "display string with hex",
				input: `%"%c3%bcsers"`,
				want: Item{
					BareItem: BareItem{Type: BareItemTypeDisplayString, DisplayString: `üsers`},
				},
			},
			{
				name:      "display string with missing starting quote",
				input:     `%hello world"`,
				wantError: ErrInvalidDisplayString,
			},
			{
				name:      "display string with missing ending quote",
				input:     `%"hello world`,
				wantError: ErrInvalidDisplayString,
			},
			{
				name:      "display string with missing hex character 1",
				input:     `%"%`,
				wantError: ErrInvalidDisplayString,
			},
			{
				name:      "display string with missing hex character 2",
				input:     `%"%c`,
				wantError: ErrInvalidDisplayString,
			},
			{
				name:      "display string with invalid hex character 1",
				input:     `%"%Cc"`,
				wantError: ErrInvalidDisplayString,
			},
			{
				name:      "display string with invalid hex character 2",
				input:     `%"%cC"`,
				wantError: ErrInvalidDisplayString,
			},
			{
				name:      "display string with invalid UTF-8",
				input:     `%"%ea"`,
				wantError: ErrInvalidDisplayString,
			},
			{
				name:  "display string with parameters",
				input: `%"hello world"; param1=%"value 1";param2=%"value 2"; param1=%"value 3"`,
				want: Item{
					BareItem: BareItem{Type: BareItemTypeDisplayString, DisplayString: `hello world`},
					Parameters: params(
						"param1", BareItem{Type: BareItemTypeDisplayString, DisplayString: `value 3`},
						"param2", BareItem{Type: BareItemTypeDisplayString, DisplayString: `value 2`},
					),
				},
			},

			{
				name:      "empty parameter key",
				input:     "123;",
				wantError: ErrInvalidKey,
			},
			{
				name:      "invalid parameter key start",
				input:     "123; A=1",
				wantError: ErrInvalidKey,
			},
			{
				name:      "invalid parameter key",
				input:     "123; aB=1",
				wantError: ErrTrailingData,
			},
			{
				name:      "invalid parameter value",
				input:     "123; a=-",
				wantError: ErrInvalidIntegerOrDecimal,
			},
			{
				name:  "boolean parameter",
				input: "123; bool; non-bool=1",
				want: Item{
					BareItem: BareItem{Type: BareItemTypeInteger, Integer: 123},
					Parameters: params(
						"bool", BareItem{Type: BareItemTypeBoolean, Boolean: true},
						"non-bool", BareItem{Type: BareItemTypeInteger, Integer: 1},
					),
				},
			},
			{
				name:  "complex parameter key",
				input: "123; *key_with-many.chars_*=1",
				want: Item{
					BareItem:   BareItem{Type: BareItemTypeInteger, Integer: 123},
					Parameters: params("*key_with-many.chars_*", BareItem{Type: BareItemTypeInteger, Integer: 1}),
				},
			},

			{
				name:  "leading spaces before value",
				input: "    123",
				want:  Item{BareItem: BareItem{Type: BareItemTypeInteger, Integer: 123}},
			},
			{
				name:  "trailing spaces after value",
				input: "123   ",
				want:  Item{BareItem: BareItem{Type: BareItemTypeInteger, Integer: 123}},
			},
			{
				name:      "trailing data after value",
				input:     "123 a",
				wantError: ErrTrailingData,
			},
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
	})

	t.Run("List", func(t *testing.T) {
		testCases := []struct {
			name      string
			input     string
			want      List
			wantError error
		}{
			{
				name: "empty",
			},

			{
				name:  "list of items",
				input: ` 123 , 123.456 , ?0 , ?1 , @123456 , token , "string" ` + "\t" + `,` + "\t" + ` %"display string" `,
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
				input: ` ( 123 123.456 ) , ( ?0  ?1 ) , ( @123456 token ) , ( "string" %"display string" ) `,
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
				wantError: ErrInvalidBareItem,
			},
			{
				name:      "tabs in inner list",
				input:     "(1\t2)",
				wantError: ErrInvalidInnerList,
			},
			{
				name:      "comma in inner list",
				input:     "(1,2)",
				wantError: ErrInvalidInnerList,
			},
			{
				name:      "invalid character after inner list item",
				input:     `(123a)`,
				wantError: ErrInvalidInnerList,
			},
			{
				name:      "unclosed inner list",
				input:     `(123`,
				wantError: ErrInvalidInnerList,
			},
			{
				name:      "unclosed inner list with parameters",
				input:     `(123; param=value`,
				wantError: ErrInvalidInnerList,
			},
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
	})
}

func BenchmarkParse(b *testing.B) {
	b.Run("Dictionary", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			_, err := Parse[Dictionary](`a=123,b=123.456,c=(?0 ?1),d=@123456,e=token,f=("string" %"display string")`)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("List", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			_, err := Parse[List](`123,123.456,(?0 ?1),@123456,token,("string" %"display string")`)
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
