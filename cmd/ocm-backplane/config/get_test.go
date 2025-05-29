package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	cliConfig "github.com/openshift/backplane-cli/pkg/cli/config"
)

var _ = Describe("Get Config Command", func() {
	var (
		tempConfigFile *os.File
		err            error
		oldStdout      *os.File
		r              *os.File
		w              *os.File
		outBuf         bytes.Buffer
	)

	// Helper function to capture stdout
	captureStdout := func() {
		oldStdout = os.Stdout
		r, w, _ = os.Pipe()
		os.Stdout = w
		outBuf.Reset()
	}

	// Helper function to restore stdout and read output
	restoreStdout := func() string {
		if w != nil {
			w.Close()
		}
		os.Stdout = oldStdout
		if r != nil {
			_, _ = outBuf.ReadFrom(r)
			r.Close()
		}
		return outBuf.String()
	}

	// Helper function to create a temporary config file
	createTempConfig := func(config cliConfig.BackplaneConfiguration) string {
		tempConfigFile, err = os.CreateTemp("", "test-config-*.json")
		Expect(err).NotTo(HaveOccurred())
		
		data, err := json.Marshal(config)
		Expect(err).NotTo(HaveOccurred())
		
		_, err = tempConfigFile.Write(data)
		Expect(err).NotTo(HaveOccurred())
		
		err = tempConfigFile.Close()
		Expect(err).NotTo(HaveOccurred())
		
		return tempConfigFile.Name()
	}
	
	// Helper function to create an invalid temporary config file
	createInvalidTempConfig := func() string {
		tempConfigFile, err = os.CreateTemp("", "invalid-config-*.json")
		Expect(err).NotTo(HaveOccurred())
		_, err = tempConfigFile.WriteString("{invalid_json_data:}")
		Expect(err).NotTo(HaveOccurred())
		err = tempConfigFile.Close()
		Expect(err).NotTo(HaveOccurred())
		return tempConfigFile.Name()
	}

	AfterEach(func() {
		if tempConfigFile != nil {
			os.Remove(tempConfigFile.Name())
			tempConfigFile = nil
		}
		os.Unsetenv("BACKPLANE_CONFIG")
		// Ensure stdout is restored even if a test panics
		if oldStdout != nil {
			os.Stdout = oldStdout
		}
		if r != nil {
			r.Close()
		}
		if w != nil {
			w.Close()
		}
	})

	Context("when config.GetBackplaneConfiguration() fails", func() {
		It("should propagate the error", func() {
			invalidConfigPath := createInvalidTempConfig()
			os.Setenv("BACKPLANE_CONFIG", invalidConfigPath)

			err := getConfig(nil, []string{"url"}) // Arg doesn't matter here
			Expect(err).To(HaveOccurred())
			// Check for a generic part of the error message that indicates parsing/loading failure
			Expect(err.Error()).Should(Or(ContainSubstring("unmarshal errors"), ContainSubstring("Unable to load backplane config")))
		})
	})

	Context("when retrieving specific configuration variables", func() {
		var mockConfig cliConfig.BackplaneConfiguration
		var configFilePath string

		BeforeEach(func() {
			// Initialize with default values, can be overridden by specific tests
			mockConfig = cliConfig.BackplaneConfiguration{
				URL:              "https://default.example.com",
				ProxyURL:         nil,
				SessionDirectory: "/default/sessions",
				PagerDutyAPIKey:  "default_pd_key",
				Govcloud:         false,
			}
		})

		It("should correctly print the 'url'", func() {
			expectedURL := "https://test.example.com"
			mockConfig.URL = expectedURL
			configFilePath = createTempConfig(mockConfig)
			os.Setenv("BACKPLANE_CONFIG", configFilePath)

			captureStdout()
			err := getConfig(nil, []string{URLConfigVar})
			output := restoreStdout()

			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal(fmt.Sprintf("%s: %s\n", URLConfigVar, expectedURL)))
		})

		It("should correctly print the 'proxy-url' when set", func() {
			expectedProxyURL := "http://proxy.example.com"
			mockConfig.ProxyURL = &expectedProxyURL // Pointer to string
			configFilePath = createTempConfig(mockConfig)
			os.Setenv("BACKPLANE_CONFIG", configFilePath)

			captureStdout()
			err := getConfig(nil, []string{ProxyURLConfigVar})
			output := restoreStdout()

			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal(fmt.Sprintf("%s: %s\n", ProxyURLConfigVar, expectedProxyURL)))
		})

		It("should correctly print an empty 'proxy-url' when not set (nil)", func() {
			mockConfig.ProxyURL = nil // Explicitly nil
			configFilePath = createTempConfig(mockConfig)
			os.Setenv("BACKPLANE_CONFIG", configFilePath)

			captureStdout()
			err := getConfig(nil, []string{ProxyURLConfigVar})
			output := restoreStdout()

			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal(fmt.Sprintf("%s: %s\n", ProxyURLConfigVar, ""))) // Expect empty string
		})

		It("should correctly print the 'session-directory'", func() {
			expectedSessionDir := "/test/sessions"
			mockConfig.SessionDirectory = expectedSessionDir
			configFilePath = createTempConfig(mockConfig)
			os.Setenv("BACKPLANE_CONFIG", configFilePath)

			captureStdout()
			err := getConfig(nil, []string{SessionConfigVar})
			output := restoreStdout()

			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal(fmt.Sprintf("%s: %s\n", SessionConfigVar, expectedSessionDir)))
		})

		It("should correctly print the 'pd-api-key'", func() {
			expectedPDKey := "test_pd_api_key_123"
			mockConfig.PagerDutyAPIKey = expectedPDKey
			configFilePath = createTempConfig(mockConfig)
			os.Setenv("BACKPLANE_CONFIG", configFilePath)

			captureStdout()
			err := getConfig(nil, []string{PagerDutyAPIConfigVar})
			output := restoreStdout()

			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal(fmt.Sprintf("%s: %s\n", PagerDutyAPIConfigVar, expectedPDKey)))
		})
		
		It("should correctly print 'govcloud' when true", func() {
			expectedGovcloud := true
			mockConfig.Govcloud = expectedGovcloud
			configFilePath = createTempConfig(mockConfig)
			os.Setenv("BACKPLANE_CONFIG", configFilePath)

			captureStdout()
			err := getConfig(nil, []string{GovcloudVar})
			output := restoreStdout()

			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal(fmt.Sprintf("%s: %t\n", GovcloudVar, expectedGovcloud)))
		})

		It("should correctly print 'govcloud' when false", func() {
			expectedGovcloud := false
			mockConfig.Govcloud = expectedGovcloud
			configFilePath = createTempConfig(mockConfig)
			os.Setenv("BACKPLANE_CONFIG", configFilePath)

			captureStdout()
			err := getConfig(nil, []string{GovcloudVar})
			output := restoreStdout()

			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal(fmt.Sprintf("%s: %t\n", GovcloudVar, expectedGovcloud)))
		})
	})

	Context("when retrieving 'all' configuration variables", func() {
		It("should correctly print all variables", func() {
			expectedURL := "https://all.example.com"
			expectedProxy := "http://allproxy.example.com"
			expectedSession := "/all/sessions"
			expectedPD := "all_pd_key"
			expectedGov := true

			mockConfig := cliConfig.BackplaneConfiguration{
				URL:              expectedURL,
				ProxyURL:         &expectedProxy,
				SessionDirectory: expectedSession,
				PagerDutyAPIKey:  expectedPD,
				Govcloud:         expectedGov,
			}
			configFilePath := createTempConfig(mockConfig)
			os.Setenv("BACKPLANE_CONFIG", configFilePath)

			captureStdout()
			err := getConfig(nil, []string{"all"})
			output := restoreStdout()

			Expect(err).NotTo(HaveOccurred())
			
			expectedOutput := fmt.Sprintf("%s: %s\n", URLConfigVar, expectedURL)
			expectedOutput += fmt.Sprintf("%s: %s\n", ProxyURLConfigVar, expectedProxy)
			expectedOutput += fmt.Sprintf("%s: %s\n", SessionConfigVar, expectedSession)
			expectedOutput += fmt.Sprintf("%s: %s\n", PagerDutyAPIConfigVar, expectedPD)
			expectedOutput += fmt.Sprintf("%s: %t\n", GovcloudVar, expectedGov)
			
			Expect(output).To(Equal(expectedOutput))
		})

		It("should correctly print all variables when proxy-url is nil", func() {
			expectedURL := "https://allnil.example.com"
			expectedSession := "/allnil/sessions"
			expectedPD := "allnil_pd_key"
			expectedGov := false

			mockConfig := cliConfig.BackplaneConfiguration{
				URL:              expectedURL,
				ProxyURL:         nil, // Key difference
				SessionDirectory: expectedSession,
				PagerDutyAPIKey:  expectedPD,
				Govcloud:         expectedGov,
			}
			configFilePath := createTempConfig(mockConfig)
			os.Setenv("BACKPLANE_CONFIG", configFilePath)

			captureStdout()
			err := getConfig(nil, []string{"all"})
			output := restoreStdout()

			Expect(err).NotTo(HaveOccurred())
			
			expectedOutput := fmt.Sprintf("%s: %s\n", URLConfigVar, expectedURL)
			expectedOutput += fmt.Sprintf("%s: %s\n", ProxyURLConfigVar, "") // Empty string for nil proxy
			expectedOutput += fmt.Sprintf("%s: %s\n", SessionConfigVar, expectedSession)
			expectedOutput += fmt.Sprintf("%s: %s\n", PagerDutyAPIConfigVar, expectedPD)
			expectedOutput += fmt.Sprintf("%s: %t\n", GovcloudVar, expectedGov)
			
			Expect(output).To(Equal(expectedOutput))
		})
	})

	Context("when an unsupported variable is requested", func() {
		It("should return an error listing supported variables", func() {
			// Setup a basic valid config, its content doesn't matter much here
			mockConfig := cliConfig.BackplaneConfiguration{URL: "https://example.com"}
			configFilePath := createTempConfig(mockConfig)
			os.Setenv("BACKPLANE_CONFIG", configFilePath)

			unsupportedVar := "nonexistent-var"
			captureStdout() // In case it prints anything before erroring
			err := getConfig(nil, []string{unsupportedVar})
			output := restoreStdout() // Capture any mistaken output

			Expect(err).To(HaveOccurred())
			expectedErrorMsg := fmt.Sprintf("supported config variables are %s, %s, %s, %s, & %s",
				URLConfigVar, ProxyURLConfigVar, SessionConfigVar, PagerDutyAPIConfigVar, GovcloudVar)
			Expect(err.Error()).To(Equal(expectedErrorMsg))
			Expect(output).To(BeEmpty(), "Should not print anything to stdout on unsupported variable error")
		})
	})
})
