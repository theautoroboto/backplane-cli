package utils

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	BackplaneApi "github.com/openshift/backplane-api/pkg/client"
)

func TestParseParamsFlag(t *testing.T) {
	tests := []struct {
		description string
		input       []string
		expectedMap map[string]string
		expectError bool
	}{
		{
			description: "Empty input slice",
			input:       []string{},
			expectedMap: map[string]string{},
			expectError: false,
		},
		{
			description: "Valid parameters",
			input:       []string{"KEY1=VAL1", "KEY2=VAL2"},
			expectedMap: map[string]string{"KEY1": "VAL1", "KEY2": "VAL2"},
			expectError: false,
		},
		{
			description: "Parameters with spaces around keys/values",
			input:       []string{"  KEY1  =  VAL1  ", "KEY2=VAL2"},
			expectedMap: map[string]string{"KEY1": "VAL1", "KEY2": "VAL2"},
			expectError: false,
		},
		{
			description: "Parameter value containing an equals sign",
			input:       []string{"KEY1=VAL1=MOREVAL"},
			expectedMap: map[string]string{"KEY1": "VAL1=MOREVAL"},
			expectError: false,
		},
		{
			description: "Parameter with empty value",
			input:       []string{"KEY1="},
			expectedMap: map[string]string{"KEY1": ""},
			expectError: false,
		},
		{
			description: "Malformed parameter (missing '=')",
			input:       []string{"KEY1VAL1"},
			expectedMap: nil, // Not checked if error is expected
			expectError: true,
		},
		{
			description: "Malformed parameter (empty key)",
			input:       []string{" =VAL1"},
			expectedMap: nil, // Not checked if error is expected
			expectError: true,
		},
		{
			description: "Malformed parameter (key with only spaces)",
			input:       []string{"   =VAL1"},
			expectedMap: nil, // Not checked if error is expected
			expectError: true,
		},
		{
			description: "Mix of valid and invalid (invalid first)",
			input:       []string{"BADPARAM", "KEY1=VAL1"},
			expectedMap: nil, // Not checked if error is expected
			expectError: true,
		},
		{
			description: "Mix of valid and invalid (valid first)",
			input:       []string{"KEY1=VAL1", "BADPARAM"},
			expectedMap: nil, // Not checked if error is expected
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			result, err := ParseParamsFlag(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected an error for input %v, but got none", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("Did not expect an error for input %v, but got: %v", tt.input, err)
				}
				if !reflect.DeepEqual(result, tt.expectedMap) {
					t.Errorf("For input %v, expected map %v, but got %v", tt.input, tt.expectedMap, result)
				}
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

func TestTryParseBackplaneAPIError(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	intPtr := func(i int) *int { return &i }

	tests := []struct {
		description         string
		responseBody        string
		responseStatus      int
		responseIsNil       bool
		expectedErrorStruct *BackplaneApi.Error
		expectError         bool
		expectedErrorSubstr string // For checking parts of the error message, especially for truncated bodies
	}{
		{
			description:    "Valid BackplaneApi.Error JSON",
			responseBody:   `{"message": "error message", "status_code": 400}`,
			responseStatus: http.StatusBadRequest,
			expectedErrorStruct: &BackplaneApi.Error{
				Message:    strPtr("error message"),
				StatusCode: intPtr(400),
			},
			expectError: false,
		},
		{
			description:         "Valid JSON but not BackplaneApi.Error structure",
			responseBody:        `{"other_error": "some other format"}`,
			responseStatus:      http.StatusInternalServerError,
			expectedErrorStruct: nil,
			expectError:         true,
			expectedErrorSubstr: "failed to unmarshal JSON response from backplane (body starts with: '{\"other_error\": \"some other format\"}')",
		},
		{
			description:         "Non-JSON body",
			responseBody:        "plain text error",
			responseStatus:      http.StatusServiceUnavailable,
			expectedErrorStruct: nil,
			expectError:         true,
			expectedErrorSubstr: "failed to unmarshal JSON response from backplane (body starts with: 'plain text error')",
		},
		{
			description:         "Non-JSON body (Japanese)",
			responseBody:        "ただのテキストエラー",
			responseStatus:      http.StatusServiceUnavailable,
			expectedErrorStruct: nil,
			expectError:         true,
			expectedErrorSubstr: "failed to unmarshal JSON response from backplane (body starts with: 'ただのテキストエラー')",
		},
		{
			description: "Long Non-JSON body that should be truncated",
			responseBody: "This is a very long plain text error that definitely exceeds the two hundred character limit that has been implemented in the TryParseBackplaneAPIError function to ensure that error messages do not become overly verbose or log excessive amounts of data from an unexpected response body. This text will be truncated.",
			responseStatus: http.StatusBadGateway,
			expectedErrorStruct: nil,
			expectError: true,
			expectedErrorSubstr: "failed to unmarshal JSON response from backplane (body starts with: 'This is a very long plain text error that definitely exceeds the two hundred character limit that has been implemented in the TryParseBackplaneAPIError function to ensure that error messages do not beco...')",
		},
		{
			description:         "Empty body",
			responseBody:        "",
			responseStatus:      http.StatusOK, // Or any other status
			expectedErrorStruct: nil,
			expectError:         true,
			expectedErrorSubstr: "failed to unmarshal JSON response from backplane (body starts with: '')",
		},
		{
			description:         "Nil http.Response",
			responseIsNil:       true,
			expectedErrorStruct: nil,
			expectError:         true,
			expectedErrorSubstr: "parse err provided nil http response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			var resp *http.Response

			if !tt.responseIsNil {
				rr := httptest.NewRecorder()
				rr.WriteString(tt.responseBody)
				rr.Code = tt.responseStatus // Set the HTTP status code
				resp = rr.Result()
				// httptest.ResponseRecorder.Result() returns a response with a body that needs to be closed
				// but TryParseBackplaneAPIError closes it. If we need to read it again, we'd need to replace it.
				// For this test, it's fine as is.
			} else {
				resp = nil
			}

			parsedError, err := TryParseBackplaneAPIError(resp)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected an error, but got nil")
				} else if tt.expectedErrorSubstr != "" && !strings.Contains(err.Error(), tt.expectedErrorSubstr) {
					t.Errorf("Error message '%s' does not contain expected substring '%s'", err.Error(), tt.expectedErrorSubstr)
				}
			} else {
				if err != nil {
					t.Errorf("Did not expect an error, but got: %v", err)
				}
			}

			if !reflect.DeepEqual(parsedError, tt.expectedErrorStruct) {
				// For more detailed diff in case of pointer mismatches in struct fields
				if parsedError != nil && tt.expectedErrorStruct != nil {
					if (parsedError.Message == nil && tt.expectedErrorStruct.Message != nil) || (parsedError.Message != nil && tt.expectedErrorStruct.Message == nil) || (parsedError.Message != nil && tt.expectedErrorStruct.Message != nil && *parsedError.Message != *tt.expectedErrorStruct.Message) {
						t.Errorf("Message mismatch: got %v, want %v", parsedError.Message, tt.expectedErrorStruct.Message)
					}
					if (parsedError.StatusCode == nil && tt.expectedErrorStruct.StatusCode != nil) || (parsedError.StatusCode != nil && tt.expectedErrorStruct.StatusCode == nil) || (parsedError.StatusCode != nil && tt.expectedErrorStruct.StatusCode != nil && *parsedError.StatusCode != *tt.expectedErrorStruct.StatusCode) {
						t.Errorf("StatusCode mismatch: got %v, want %v", parsedError.StatusCode, tt.expectedErrorStruct.StatusCode)
					}
				} else {
					t.Errorf("Parsed error struct mismatch: got %+v, want %+v", parsedError, tt.expectedErrorStruct)
				}
			}
		})
	}
}
