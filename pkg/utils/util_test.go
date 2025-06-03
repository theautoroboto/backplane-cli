package utils

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestParseParamFlag(t *testing.T) {
	tests := []struct {
		inp    []string
		expect map[string]string
		expErr bool
	}{
		{
			inp:    []string{"k1=v1"},
			expect: map[string]string{"k1": "v1"},
			expErr: false,
		},
		{
			inp:    []string{"k1=v1", "k2=v2"},
			expect: map[string]string{"k1": "v1", "k2": "v2"},
			expErr: false,
		},
		{
			inp:    []string{"k1=v1", "k1=v2"},
			expect: map[string]string{"k1": "v2"},
			expErr: false,
		},
		{
			inp:    []string{"k1"},
			expect: nil,
			expErr: true,
		},
		{
			inp:    []string{"k1="},
			expect: map[string]string{"k1": ""},
			expErr: false,
		},
	}

	for n, tt := range tests {
		t.Run(fmt.Sprintf("case %d", n), func(t *testing.T) {
			result, err := ParseParamsFlag(tt.inp)
			if !reflect.DeepEqual(result, tt.expect) {
				t.Errorf("Expecting: %s, but get: %s", tt.expect, result)
			}
			if tt.expErr && err == nil {
				t.Errorf("Expecting error but got none")
			}
		})
	}
}

func TestGetFreePort(t *testing.T) {
	port, err := GetFreePort()
	if err != nil {
		t.Errorf("unable get port")
	}
	if port <= 1024 || port > 65535 {
		t.Errorf("unexpected port %d", port)
	}
}

func TestMatchBaseDomain(t *testing.T) {
	tests := []struct {
		name       string
		longURL    string
		baseDomain string
		expect     bool
	}{
		{
			name:       "case-1",
			longURL:    "a.example.com",
			baseDomain: "example.com",
			expect:     true,
		},
		{
			name:       "case-2",
			longURL:    "a.b.c.example.com",
			baseDomain: "example.com",
			expect:     true,
		},
		{
			name:       "case-3",
			longURL:    "example.com",
			baseDomain: "example.com",
			expect:     true,
		},
		{
			name:       "case-4",
			longURL:    "a.example.com",
			baseDomain: "",
			expect:     true,
		},
		{
			name:       "case-5",
			longURL:    "",
			baseDomain: "",
			expect:     true,
		},
		{
			name:       "case-6",
			longURL:    "",
			baseDomain: "example.com",
			expect:     false,
		},
		{
			name:       "case-7",
			longURL:    "a.example.com.io",
			baseDomain: "example.com",
			expect:     false,
		},
		{
			name:       "case-8",
			longURL:    "a.b.c",
			baseDomain: "e.f.g",
			expect:     false,
		},
		{
			name:       "case-9",
			longURL:    "a",
			baseDomain: "a",
			expect:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchBaseDomain(tt.longURL, tt.baseDomain)
			if result != tt.expect {
				t.Errorf("Expecting: %t, but get: %t", tt.expect, result)
			}
		})
	}
}

func TestSanitizeRoleSessionName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "ValidNoChange",
			input: "user@example.com",
			want:  "user@example.com",
		},
		{
			name:  "InvalidChars",
			input: "user!name@example.com#test",
			want:  "user_name@example.com_test",
		},
		{
			name:  "TooLong",
			input: "longstringlongstringlongstringlongstringlongstringlongstringlongstring", // 70 chars
			want:  "longstringlongstringlongstringlongstringlongstringlongstringlong",   // Truncated to 64
		},
		{
			name:  "TooLongWithInvalid",
			input: "longstring!longstring?longstringlongstringlongstringlongstringlong", // 70 chars with invalid
			want:  "longstring_longstring_longstringlongstringlongstringlongstringlo",   // Sanitized then truncated
		},
		{
			name:  "TooShort_Prefix_WasNotEmptyValidChar",
			input: "a",
			want:  "rs-a",
		},
		{
			name:  "TooShort_Prefix_WasNotEmptyInvalidChar",
			input: "!",
			want:  "rs-_",
		},
		{
			name:  "TooShort_Prefix_MultipleInvalidChars_ShouldBe_ValidSpecialChar",
			input: "!@#", // '!' -> '_', '@' is valid, '#' -> '_' => "_@_" (len 3, not < 2)
			want:  "_@_",
		},
		{
			name:  "TooShort_Default_BecomesEmpty",
			input: "!!", // becomes "__", which is valid length
			want:  "__",
		},
		{
			name:  "TooShort_Default_SanitizedToEmptyThenPrefixedStillTooShort",
			input: " ", // becomes "_", then "rs-_", which is fine.
			want:  "rs-_",
		},
		{
			name:  "EmptyInput",
			input: "",
			want:  "default_session_name",
		},
		{
			name:  "ExactMaxLength",
			input: strings.Repeat("a", 64),
			want:  strings.Repeat("a", 64),
		},
		{
			name:  "ExactMinLength",
			input: "ab",
			want:  "ab",
		},
		{
			name:  "AllAllowedSpecialChars",
			input: "user+name=bob,foo.bar@test-domain_corp",
			want:  "user+name=bob,foo.bar@test-domain_corp",
		},
		{
			name:  "StartsEndsWithInvalid",
			input: "!username@example.com!",
			want:  "_username@example.com_",
		},
		{
			name:  "OnlyInvalidChars_Short",
			input: "!!!###$$$", // 9 chars, becomes "_________"
			want:  "_________",
		},
		{
			name:  "OnlyInvalidChars_Long",
			input: strings.Repeat("!", 70), // 70 "!"
			want:  strings.Repeat("_", 64), // 64 "_"
		},
		{
			name:  "OnlyInvalidChars_BecomesTooShort",
			input: "#", // becomes "_", then "rs-_"
			want:  "rs-_",
		},
		{
			name:  "SingleCharValid_ForPrefixCase", // Duplicate of TooShort_Prefix_WasNotEmptyValidChar
			input: "a",
			want:  "rs-a",
		},
		{
			name:  "SingleCharInvalid_ForPrefixCase", // Duplicate of TooShort_Prefix_WasNotEmptyInvalidChar
			input: "!",
			want:  "rs-_",
		},
		{
			name:  "ValidLongStringNoChange_63chars",
			input: strings.Repeat("a", 63),
			want:  strings.Repeat("a", 63),
		},
		{
			name:  "ValidButResultsInTooShortAfterSanitizationAndTruncation", // This test name is not quite right anymore, it's just a valid case.
			input: "a" + strings.Repeat("!", 70), // "a________________... (63 times)" -> len 64
			want:  "a" + strings.Repeat("_", 63),
		},
		{
			name:  "EndsUpAsDefaultNameFromShortInvalid",
			input: " ", // becomes "_", then "rs-_"
			want:  "rs-_",
		},
		{
			name:  "EndsUpAsDefaultNameFromEmpty", // Same as EmptyInput
			input: "",
			want:  "default_session_name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeRoleSessionName(tt.input); got != tt.want {
				t.Errorf("SanitizeRoleSessionName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
