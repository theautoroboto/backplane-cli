package testjob

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"sigs.k8s.io/kustomize/api/konfig"

	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"

	"github.com/openshift/backplane-cli/pkg/backplaneapi"
	backplaneapiMock "github.com/openshift/backplane-cli/pkg/backplaneapi/mocks"
	"github.com/openshift/backplane-cli/pkg/client/mocks"
	"github.com/openshift/backplane-cli/pkg/info"
	"github.com/openshift/backplane-cli/pkg/ocm"
	ocmMock "github.com/openshift/backplane-cli/pkg/ocm/mocks"
	"github.com/openshift/backplane-cli/pkg/utils"
)

const (
	MetadataYaml = `
file: script.sh
name: example
description: just an example
author: dude
allowedGroups: 
  - SREP
rbac:
    roles:
      - namespace: "kube-system"
        rules:
          - verbs:
            - "*"
            apiGroups:
            - ""
            resources:
            - "*"
            resourceNames:
            - "*"
    clusterRoleRules:
        - verbs:
            - "*"
          apiGroups:
            - ""
          resources:
            - "*"
          resourceNames:
            - "*"
language: bash
`
)

var _ = Describe("testJob create command", func() {

	var (
		mockCtrl         *gomock.Controller
		mockClient       *mocks.MockClientInterface
		mockOcmInterface *ocmMock.MockOCMInterface
		mockClientUtil   *backplaneapiMock.MockClientUtils

		testClusterID string
		testToken     string
		trueClusterID string
		proxyURI      string
		tempDir       string
		sourceDir     string
		workingDir    string

		fakeResp *http.Response

		sut    *cobra.Command
		ocmEnv *cmv1.Environment
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = mocks.NewMockClientInterface(mockCtrl)

		workingDir = konfig.CurrentWorkingDir()

		tempDir, _ = os.MkdirTemp("", "createJobTest")

		_ = os.WriteFile(path.Join(tempDir, "metadata.yaml"), []byte(MetadataYaml), 0600)
		_ = os.WriteFile(path.Join(tempDir, "script.sh"), []byte("echo hello"), 0600)

		_ = os.Chdir(tempDir)

		mockOcmInterface = ocmMock.NewMockOCMInterface(mockCtrl)
		ocm.DefaultOCMInterface = mockOcmInterface

		mockClientUtil = backplaneapiMock.NewMockClientUtils(mockCtrl)
		backplaneapi.DefaultClientUtils = mockClientUtil

		testClusterID = "test123"
		testToken = "hello123"
		trueClusterID = "trueID123"
		proxyURI = "https://shard.apps"

		sut = NewTestJobCommand()

		fakeResp = &http.Response{
			Body: MakeIoReader(`
{"testId":"tid",
"logs":"",
"message":"",
"status":"Pending"
}
`),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeResp.Header.Add("Content-Type", "json")
		os.Setenv(info.BackplaneURLEnvName, proxyURI)
		ocmEnv, _ = cmv1.NewEnvironment().BackplaneURL("https://dummy.api").Build()
	})

	AfterEach(func() {
		os.Setenv(info.BackplaneURLEnvName, "")
		_ = os.RemoveAll(tempDir)
		// Clear kube config file
		utils.RemoveTempKubeConfig()
		mockCtrl.Finish()
	})

	Context("create test job", func() {
		It("when running with a simple case should work as expected", func() {

			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			// It should query for the internal cluster id first
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)
			mockClient.EXPECT().CreateTestScriptRun(gomock.Any(), trueClusterID, gomock.Any()).Return(fakeResp, nil)

			sut.SetArgs([]string{"create", "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should respect url flag", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			// It should query for the internal cluster id first
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient("https://newbackplane.url").Return(mockClient, nil)
			mockClient.EXPECT().CreateTestScriptRun(gomock.Any(), trueClusterID, gomock.Any()).Return(fakeResp, nil)

			sut.SetArgs([]string{"create", "--cluster-id", testClusterID, "--url", "https://newbackplane.url"})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should respect the base image when supplied as a flag", func() {

			baseImgOverride := "quay.io/foo/bar"
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			// It should query for the internal cluster id first
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClient.EXPECT().CreateTestScriptRun(gomock.Any(), trueClusterID, gomock.Any()).Return(fakeResp, nil)

			sut.SetArgs([]string{"create", "--cluster-id", testClusterID, "--base-image-override", baseImgOverride})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should be able to use the specified script dir", func() {

			sourceDir, _ = os.MkdirTemp("", "manualScriptDir")
			_ = os.WriteFile(path.Join(sourceDir, "metadata.yaml"), []byte(MetadataYaml), 0600)
			_ = os.WriteFile(path.Join(sourceDir, "script.sh"), []byte("echo hello"), 0600)
			defer os.RemoveAll(sourceDir)

			_ = os.Chdir(workingDir)

			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			// It should query for the internal cluster id first
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)
			mockClient.EXPECT().CreateTestScriptRun(gomock.Any(), trueClusterID, gomock.Any()).Return(fakeResp, nil)

			sut.SetArgs([]string{"create", "--cluster-id", testClusterID, "--source-dir", sourceDir})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should return with correct error message when the given source dir is incorrect", func() {
			nonExistDir := "testDir"
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			// It should query for the internal cluster id first
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)

			sut.SetArgs([]string{"create", "--cluster-id", testClusterID, "--source-dir", nonExistDir})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("does not exist or it is not a directory"))
		})

		It("Should able use the current logged in cluster if non specified and retrieve from config file", func() {
			os.Setenv(info.BackplaneURLEnvName, "https://api-backplane.apps.something.com")
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			err := utils.CreateTempKubeConfig(nil)
			Expect(err).To(BeNil())
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq("configcluster")).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient("https://api-backplane.apps.something.com").Return(mockClient, nil)
			mockClient.EXPECT().CreateTestScriptRun(gomock.Any(), "configcluster", gomock.Any()).Return(fakeResp, nil)

			sut.SetArgs([]string{"create"})
			err = sut.Execute()

			Expect(err).To(BeNil())
		})

		It("should fail when backplane did not return a 200", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)
			mockClient.EXPECT().CreateTestScriptRun(gomock.Any(), trueClusterID, gomock.Any()).Return(nil, errors.New("err"))

			sut.SetArgs([]string{"create", "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})

		It("should fail when backplane returns a non parsable response", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)
			fakeResp.Body = MakeIoReader("Sad")
			mockClient.EXPECT().CreateTestScriptRun(gomock.Any(), trueClusterID, gomock.Any()).Return(fakeResp, errors.New("err"))

			sut.SetArgs([]string{"create", "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})

		It("should fail when metadata is not found/invalid", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)

			_ = os.Remove(path.Join(tempDir, "metadata.yaml"))

			sut.SetArgs([]string{"create", "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})

		It("should fail when script file is not found/invalid", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)

			_ = os.Remove(path.Join(tempDir, "script.sh"))

			sut.SetArgs([]string{"create", "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})

		It("should not run in production environment", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsProduction().Return(true, nil)

			_ = os.Remove(path.Join(tempDir, "script.sh"))

			sut.SetArgs([]string{"create", "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})

		It("should not run in production environment", func() {
			mockOcmInterface.EXPECT().IsProduction().Return(true, nil)

			_ = os.Remove(path.Join(tempDir, "script.sh"))

			sut.SetArgs([]string{"create", "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).ToNot(BeNil())
		})

		It("should import and inline a library file in the same directory", func() {
			script := `#!/bin/bash
set -eo pipefail

source /managed-scripts/lib.sh

echo_touch "Hello"
`
			lib := fmt.Sprintf(`function echo_touch () {
    echo $1 > %s/ran_function
}
`, tempDir)

			GetGitRepoPath = exec.Command("echo", tempDir)
			// tmp/createJobTest3397561583
			_ = os.WriteFile(path.Join(tempDir, "script.sh"), []byte(script), 0600)
			_ = os.Mkdir(path.Join(tempDir, "scripts"), 0755)
			_ = os.WriteFile(path.Join(tempDir, "scripts", "lib.sh"), []byte(lib), 0600)
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsProduction().Return(false, nil)
			// It should query for the internal cluster id first
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClient(proxyURI).Return(mockClient, nil)
			mockClient.EXPECT().CreateTestScriptRun(gomock.Any(), trueClusterID, gomock.Any()).Return(fakeResp, nil)

			sut.SetArgs([]string{"create", "--cluster-id", testClusterID})
			err := sut.Execute()

			Expect(err).To(BeNil())
		})
	})
})

func TestInlineLibrarySourceFiles(t *testing.T) {
	originalGetGitRepoPath := GetGitRepoPath

	setup := func(t *testing.T) (string, string) {
		// Create a temporary directory to act as the main script's location
		scriptDir := t.TempDir()
		// Create a temporary directory to act as the git repo root
		repoRootDir := t.TempDir()

		// Mock GetGitRepoPath to return our temporary repo root
		GetGitRepoPath = exec.Command("echo", repoRootDir)
		t.Cleanup(func() { GetGitRepoPath = originalGetGitRepoPath })

		return scriptDir, repoRootDir
	}

	t.Run("No source line", func(t *testing.T) {
		scriptDir, _ := setup(t)
		scriptPath := filepath.Join(scriptDir, "main.sh")
		originalScriptContent := "#!/bin/bash\necho \"Hello, world!\"\n"
		
		result, err := inlineLibrarySourceFiles(originalScriptContent, scriptPath)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if result != originalScriptContent {
			t.Errorf("Expected script content to be unchanged, got:\n%s\nWant:\n%s", result, originalScriptContent)
		}
	})

	t.Run("Valid source line", func(t *testing.T) {
		scriptDir, repoRootDir := setup(t)
		scriptPath := filepath.Join(scriptDir, "main.sh")

		// Create the library file
		libDir := filepath.Join(repoRootDir, "scripts", "libs")
		if err := os.MkdirAll(libDir, 0755); err != nil {
			t.Fatalf("Failed to create lib dir: %v", err)
		}
		libPath := filepath.Join(libDir, "my-lib.sh")
		libContent := "echo \"hello from lib\""
		if err := os.WriteFile(libPath, []byte(libContent), 0644); err != nil {
			t.Fatalf("Failed to write lib file: %v", err)
		}

		originalScriptContent := "#!/bin/bash\nsource /managed-scripts/libs/my-lib.sh\necho \"main script\"\n"
		
		result, err := inlineLibrarySourceFiles(originalScriptContent, scriptPath)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		expectedLibEncoded := base64.StdEncoding.EncodeToString([]byte(libContent))
		expectedSnippetPart := fmt.Sprintf(`echo "%s" | base64 -d > "$TMP_LIB_FILE"`, expectedLibEncoded)

		if !strings.Contains(result, "TMP_LIB_FILE=$(mktemp /tmp/backplane-lib-XXXXXX.sh)") {
			t.Errorf("Result does not contain mktemp command. Got:\n%s", result)
		}
		if !strings.Contains(result, expectedSnippetPart) {
			t.Errorf("Result does not contain correct base64 decode and redirect. Got:\n%s", result)
		}
		if !strings.Contains(result, `source "$TMP_LIB_FILE"`) {
			t.Errorf("Result does not source the temp file. Got:\n%s", result)
		}
		if !strings.Contains(result, `rm "$TMP_LIB_FILE"`) {
			t.Errorf("Result does not remove the temp file. Got:\n%s", result)
		}
		if strings.Contains(result, "source /managed-scripts/libs/my-lib.sh") {
			t.Errorf("Original source line was not replaced. Got:\n%s", result)
		}
		if !strings.HasSuffix(strings.TrimSpace(result), "echo \"main script\"") {
            t.Errorf("Main script content altered or misplaced. Got:\n%s", result)
        }
	})

	t.Run("Library does not exist", func(t *testing.T) {
		scriptDir, repoRootDir := setup(t) // repoRootDir will be empty of the specific lib
		scriptPath := filepath.Join(scriptDir, "main.sh")

		// Ensure scripts/libs directory exists, but not the file itself
		libDir := filepath.Join(repoRootDir, "scripts", "libs")
		if err := os.MkdirAll(libDir, 0755); err != nil {
			t.Fatalf("Failed to create lib dir: %v", err)
		}
		
		originalScriptContent := "source /managed-scripts/libs/nonexistent-lib.sh\n"
		
		_, err := inlineLibrarySourceFiles(originalScriptContent, scriptPath)
		if err == nil {
			t.Fatalf("Expected an error for non-existent library, got nil")
		}
		if !strings.Contains(err.Error(), "no such file or directory") && !strings.Contains(err.Error(), "cannot find the path specified") { // OS-dependent error messages
            t.Errorf("Expected 'no such file' error, got: %v", err)
        }
	})

	t.Run("GetGitRepoPath fails", func(t *testing.T) {
		scriptDir, _ := setup(t)
		scriptPath := filepath.Join(scriptDir, "main.sh")

		// Mock GetGitRepoPath to return an error
		originalGetGitRepoPath := GetGitRepoPath // Store original
		GetGitRepoPath = exec.Command("false") // Command that will fail
		t.Cleanup(func() { GetGitRepoPath = originalGetGitRepoPath }) // Restore
		
		originalScriptContent := "source /managed-scripts/libs/any-lib.sh\n"
		
		_, err := inlineLibrarySourceFiles(originalScriptContent, scriptPath)
		if err == nil {
			t.Fatalf("Expected an error from GetGitRepoPath, got nil")
		}
		// Check if the error is an exit error, which is what exec.Command("false").Run() would produce
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			// If it's not an ExitError, it might be some other error from Run(), like command not found.
			// For this test, we just care that an error is propagated.
			// A more specific check might be needed if the mock was more complex.
			t.Logf("Note: GetGitRepoPath failed as expected, but not with an ExitError. Error: %v", err)
		}
	})
}
