package login

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/trivago/tgo/tcontainer"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/andygrunwald/go-jira"
	"github.com/openshift/backplane-cli/pkg/backplaneapi"
	backplaneapiMock "github.com/openshift/backplane-cli/pkg/backplaneapi/mocks"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/client/mocks"
	jiraClient "github.com/openshift/backplane-cli/pkg/jira"
	jiraMock "github.com/openshift/backplane-cli/pkg/jira/mocks"
	"github.com/openshift/backplane-cli/pkg/login"
	"github.com/openshift/backplane-cli/pkg/ocm"
	ocmMock "github.com/openshift/backplane-cli/pkg/ocm/mocks"
	"github.com/openshift/backplane-cli/pkg/utils"
)

func MakeIoReader(s string) io.ReadCloser {
	r := io.NopCloser(strings.NewReader(s)) // r type is io.ReadCloser
	return r
}

var _ = Describe("Login command", func() {

	var (
		mockCtrl           *gomock.Controller
		mockClient         *mocks.MockClientInterface
		mockClientWithResp *mocks.MockClientWithResponsesInterface
		mockOcmInterface   *ocmMock.MockOCMInterface
		mockClientUtil     *backplaneapiMock.MockClientUtils
		mockIssueService   *jiraMock.MockIssueServiceInterface

		testClusterID            string
		testToken                string
		trueClusterID            string
		managingClusterID        string
		backplaneAPIURI          string
		serviceClusterID         string
		serviceClusterName       string
		fakeResp                 *http.Response
		ocmEnv                   *cmv1.Environment
		kubeConfigPath           string
		mockCluster              *cmv1.Cluster
		backplaneConfiguration   config.BackplaneConfiguration
		falsePagerDutyAPITkn     string
		truePagerDutyAPITkn      string
		falsePagerDutyIncidentID string
		truePagerDutyIncidentID  string
		bpConfigPath             string
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = mocks.NewMockClientInterface(mockCtrl)
		mockClientWithResp = mocks.NewMockClientWithResponsesInterface(mockCtrl)

		mockOcmInterface = ocmMock.NewMockOCMInterface(mockCtrl)
		ocm.DefaultOCMInterface = mockOcmInterface

		mockClientUtil = backplaneapiMock.NewMockClientUtils(mockCtrl)
		backplaneapi.DefaultClientUtils = mockClientUtil

		testClusterID = "test123"
		testToken = "hello123"
		trueClusterID = "trueID123"
		managingClusterID = "managingID123"
		serviceClusterID = "hs-sc-123456"
		serviceClusterName = "hs-sc-654321"
		backplaneAPIURI = "https://shard.apps"
		kubeConfigPath = "filepath"
		falsePagerDutyAPITkn = "token123"
		// nolint:gosec truePagerDutyAPIToken refers to the Test API Token provided by https://developer.pagerduty.com/api-reference
		truePagerDutyAPITkn = "y_NbAkKc66ryYTWUXYEu"
		falsePagerDutyIncidentID = "incident123"
		truePagerDutyIncidentID = "Q0ZNH7TDQBOO54"

		mockClientWithResp.EXPECT().LoginClusterWithResponse(gomock.Any(), gomock.Any()).Return(nil, nil).Times(0)

		fakeResp = &http.Response{
			Body:       MakeIoReader(`{"proxy_uri":"proxy", "statusCode":200, "message":"msg"}`),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeResp.Header.Add("Content-Type", "json")

		// Clear config file
		_ = clientcmd.ModifyConfig(clientcmd.NewDefaultPathOptions(), api.Config{}, true)
		clientcmd.UseModifyConfigLock = false

		globalOpts.BackplaneURL = backplaneAPIURI

		ocmEnv, _ = cmv1.NewEnvironment().BackplaneURL("https://dummy.api").Build()

		mockCluster = &cmv1.Cluster{}

		backplaneConfiguration = config.BackplaneConfiguration{URL: backplaneAPIURI}

		loginType = LoginTypeClusterID
	})

	AfterEach(func() {
		globalOpts.Manager = false
		globalOpts.Service = false
		globalOpts.BackplaneURL = ""
		globalOpts.ProxyURL = ""
		os.Setenv("HTTPS_PROXY", "")
		os.Unsetenv("BACKPLANE_CONFIG")
		os.Remove(bpConfigPath)
		mockCtrl.Finish()
		utils.RemoveTempKubeConfig()
	})

	Context("check ocm token", func() {

		It("Should fail when failed to get OCM token", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(nil, errors.New("err")).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()

			err := runLogin(nil, []string{testClusterID})

			Expect(err).ToNot(BeNil())
		})

		It("should save ocm token to kube config", func() {
			err := utils.CreateTempKubeConfig(nil)
			Expect(err).To(BeNil())
			globalOpts.ProxyURL = "https://squid.myproxy.com"
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockClientUtil.EXPECT().SetClientProxyURL(globalOpts.ProxyURL).Return(nil)
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterID)).Return(fakeResp, nil)

			err = runLogin(nil, []string{testClusterID})

			Expect(err).To(BeNil())

			cfg, err := utils.ReadKubeconfigRaw()

			Expect(err).To(BeNil())
			Expect(cfg.AuthInfos["anonymous"].Token).To(Equal(testToken))
		})
	})

	Context("check BP-api and proxy connection", func() {

		It("should use the specified backplane url if passed", func() {
			err := utils.CreateTempKubeConfig(nil)
			Expect(err).To(BeNil())
			globalOpts.BackplaneURL = "https://sadge.app"
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken("https://sadge.app", testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterID)).Return(fakeResp, nil)

			err = runLogin(nil, []string{testClusterID})

			Expect(err).To(BeNil())

			cfg, err := utils.ReadKubeconfigRaw()
			Expect(err).To(BeNil())
			Expect(cfg.CurrentContext).To(Equal("default/test123/anonymous"))
			Expect(len(cfg.Contexts)).To(Equal(1))
			Expect(cfg.Contexts["default/test123/anonymous"].Cluster).To(Equal(testClusterID))
			Expect(cfg.Contexts["default/test123/anonymous"].Namespace).To(Equal("default"))
		})

		It("should use the specified proxy url if passed", func() {
			err := utils.CreateTempKubeConfig(nil)
			Expect(err).To(BeNil())
			globalOpts.ProxyURL = "https://squid.myproxy.com"
			os.Setenv("HTTPS_PROXY", "https://squid.myproxy.com")
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockClientUtil.EXPECT().SetClientProxyURL(globalOpts.ProxyURL).Return(nil)
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterID)).Return(fakeResp, nil)

			err = runLogin(nil, []string{testClusterID})

			Expect(err).To(BeNil())

			cfg, err := utils.ReadKubeconfigRaw()

			Expect(err).To(BeNil())
			Expect(cfg.CurrentContext).To(Equal("default/test123/anonymous"))
			Expect(len(cfg.Contexts)).To(Equal(1))
			Expect(cfg.Contexts["default/test123/anonymous"].Cluster).To(Equal(testClusterID))
			Expect(cfg.Clusters[testClusterID].ProxyURL).To(Equal(globalOpts.ProxyURL))
			Expect(cfg.Contexts["default/test123/anonymous"].Namespace).To(Equal("default"))
		})

		It("should fail if unable to create api client", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(nil, errors.New("err"))

			err := runLogin(nil, []string{testClusterID})

			Expect(err).ToNot(BeNil())
		})

		It("should fail if ocm env backplane url is empty", func() {
			ocmEnv, _ = cmv1.NewEnvironment().Build()

			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			err := runLogin(nil, []string{testClusterID})

			Expect(err).ToNot(BeNil())
		})

	})

	Context("check cluster login", func() {
		BeforeEach(func() {
			globalOpts.Manager = false
			globalOpts.Service = false
			args.clusterInfo = false

		})

		It("when display cluster info is set to true", func() {
			err := utils.CreateTempKubeConfig(nil)
			args.defaultNamespace = "default"
			args.clusterInfo = true
			Expect(err).To(BeNil())
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterID)).Return(fakeResp, nil)
			mockOcmInterface.EXPECT().GetClusterInfoByID(gomock.Any()).Return(mockCluster, nil).Times(2)
			mockOcmInterface.EXPECT().SetupOCMConnection().Return(nil, nil)
			mockOcmInterface.EXPECT().IsClusterAccessProtectionEnabled(gomock.Any(), trueClusterID).Return(false, nil)

			err = runLogin(nil, []string{testClusterID})

			Expect(err).To(BeNil())
		})

		It("should fail to print cluster info when is set to true and PrintClusterInfo returns an error", func() {
			err := utils.CreateTempKubeConfig(nil)
			Expect(err).To(BeNil())
			args.defaultNamespace = "default"
			args.clusterInfo = true

			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterID)).Return(fakeResp, nil).AnyTimes()

			// Mock PrintClusterInfo to return an error
			mockOcmInterface.EXPECT().GetClusterInfoByID(gomock.Any()).Return(nil, errors.New("mock error"))

			err = runLogin(nil, []string{testClusterID})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to print cluster info"))
		})

		It("when running with a simple case should work as expected", func() {
			err := utils.CreateTempKubeConfig(nil)
			Expect(err).To(BeNil())
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterID)).Return(fakeResp, nil)

			err = runLogin(nil, []string{testClusterID})

			Expect(err).To(BeNil())

			cfg, err := utils.ReadKubeconfigRaw()
			Expect(err).To(BeNil())
			Expect(cfg.CurrentContext).To(Equal("default/test123/anonymous"))
			Expect(len(cfg.Contexts)).To(Equal(1))
			Expect(cfg.Contexts["default/test123/anonymous"].Cluster).To(Equal(testClusterID))
			Expect(cfg.Contexts["default/test123/anonymous"].Namespace).To(Equal("default"))
		})

		It("when the namespace of the context is passed as an argument", func() {
			err := utils.CreateTempKubeConfig(nil)
			args.defaultNamespace = "default"
			Expect(err).To(BeNil())
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterID)).Return(fakeResp, nil)

			err = runLogin(nil, []string{testClusterID})

			Expect(err).To(BeNil())

			cfg, err := utils.ReadKubeconfigRaw()
			Expect(err).To(BeNil())
			Expect(cfg.CurrentContext).To(Equal("default/test123/anonymous"))
			Expect(len(cfg.Contexts)).To(Equal(1))
			Expect(cfg.Contexts["default/test123/anonymous"].Cluster).To(Equal(testClusterID))
			Expect(cfg.Contexts["default/test123/anonymous"].Namespace).To(Equal(args.defaultNamespace))
		})

		It("Should fail when trying to find a non existent cluster", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()

			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return("", "", errors.New("err"))

			err := runLogin(nil, []string{testClusterID})

			Expect(err).ToNot(BeNil())
		})

		It("should return the managing cluster if one is requested", func() {
			globalOpts.Manager = true
			mockOcmInterface.EXPECT().GetClusterInfoByID(gomock.Any()).Return(mockCluster, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().GetManagingCluster(trueClusterID).Return(managingClusterID, managingClusterID, true, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(managingClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(managingClusterID)).Return(fakeResp, nil)

			err := runLogin(nil, []string{testClusterID})

			Expect(err).To(BeNil())
		})

		It("should failed if managing cluster not exist in same env", func() {
			globalOpts.Manager = true
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().GetManagingCluster(trueClusterID).Return(
				managingClusterID,
				managingClusterID,
				false,
				fmt.Errorf("failed to find management cluster for cluster %s in %s env", testClusterID, "http://test.env"),
			)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(managingClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(managingClusterID)).Return(fakeResp, nil).AnyTimes()

			err := runLogin(nil, []string{testClusterID})

			Expect(err).NotTo(BeNil())

			Expect(err.Error()).Should(ContainSubstring("failed to find management cluster for cluster test123 in http://test.env env"))
		})

		It("should return the service cluster if hosted cluster is given", func() {
			globalOpts.Service = true
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().GetManagingCluster(trueClusterID).Return(managingClusterID, managingClusterID, true, nil).AnyTimes() // isHostedControlPlane = true
			mockOcmInterface.EXPECT().GetServiceCluster(trueClusterID).Return(serviceClusterID, serviceClusterName, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(serviceClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(serviceClusterID)).Return(fakeResp, nil)

			err := runLogin(nil, []string{testClusterID})

			Expect(err).To(BeNil())
		})

		It("should login to current cluster if cluster id not provided", func() {
			loginType = LoginTypeExistingKubeConfig
			err := utils.CreateTempKubeConfig(nil)
			Expect(err).To(BeNil())
			globalOpts.ProxyURL = "https://squid.myproxy.com"
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockClientUtil.EXPECT().SetClientProxyURL(globalOpts.ProxyURL).Return(nil)
			mockOcmInterface.EXPECT().GetTargetCluster("configcluster").Return(testClusterID, "dummy_cluster", nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(testClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(testClusterID)).Return(fakeResp, nil)

			err = runLogin(nil, nil)

			Expect(err).To(BeNil())

			cfg, err := utils.ReadKubeconfigRaw()
			Expect(err).To(BeNil())

			Expect(cfg.Contexts["default/test123/anonymous"].Cluster).To(Equal("dummy_cluster"))
			Expect(cfg.Contexts["default/test123/anonymous"].Namespace).To(Equal("default"))
		})

		It("should fail when a proxy or backplane url is unreachable", func() {

			err := utils.CreateTempKubeConfig(nil)
			Expect(err).To(BeNil())
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterID)).Return(fakeResp, errors.New("dial tcp: lookup yourproxy.com: no such host"))

			err = runLogin(nil, []string{testClusterID})

			Expect(err).NotTo(BeNil())

		})

		It("Check KUBECONFIG when logging into multiple clusters.", func() {
			globalOpts.Manager = false
			args.multiCluster = true
			err := utils.ModifyTempKubeConfigFileName(trueClusterID)
			Expect(err).To(BeNil())

			kubePath, err := os.MkdirTemp("", ".kube")
			Expect(err).To(BeNil())

			err = login.SetKubeConfigBasePath(kubePath)
			Expect(err).To(BeNil())

			_, err = login.CreateClusterKubeConfig(trueClusterID, utils.GetDefaultKubeConfig())
			Expect(err).To(BeNil())

			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterID)).Return(fakeResp, nil)

			err = runLogin(nil, []string{testClusterID})

			Expect(os.Getenv("KUBECONFIG")).Should(ContainSubstring(trueClusterID))
			Expect(err).To(BeNil())

		})

		It("should fail if specify kubeconfigpath but not in multicluster mode", func() {
			globalOpts.Manager = false
			args.multiCluster = false
			args.kubeConfigPath = kubeConfigPath

			err := login.SetKubeConfigBasePath(args.kubeConfigPath)
			Expect(err).To(BeNil())

			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)

			err = runLogin(nil, []string{testClusterID})

			Expect(err.Error()).Should(ContainSubstring("can't save the kube config into a specific location if multi-cluster is not enabled"))

		})

		It("should fail to create PD API client and return HTTP status code 401 when unauthorized", func() {
			loginType = LoginTypePagerduty
			args.pd = truePagerDutyIncidentID

			err := utils.CreateTempKubeConfig(nil)
			Expect(err).To(BeNil())

			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()

			// Create a temporary JSON configuration file in the temp directory for testing purposes.
			tempDir := os.TempDir()
			bpConfigPath = filepath.Join(tempDir, "mock.json")
			tempFile, err := os.Create(bpConfigPath)
			Expect(err).To(BeNil())

			testData := config.BackplaneConfiguration{
				URL:              backplaneAPIURI,
				ProxyURL:         new(string),
				SessionDirectory: "",
				AssumeInitialArn: "",
				PagerDutyAPIKey:  falsePagerDutyAPITkn,
			}

			// Marshal the testData into JSON format and write it to tempFile.
			jsonData, err := json.Marshal(testData)
			Expect(err).To(BeNil())
			_, err = tempFile.Write(jsonData)
			Expect(err).To(BeNil())

			os.Setenv("BACKPLANE_CONFIG", bpConfigPath)

			err = runLogin(nil, nil)

			Expect(err.Error()).Should(ContainSubstring("status code 401"))
		})

		It("should return error when trying to login via PD but the PD API Key is not configured", func() {
			loginType = LoginTypePagerduty
			args.pd = truePagerDutyIncidentID

			err := utils.CreateTempKubeConfig(nil)
			Expect(err).To(BeNil())

			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()

			// Create a temporary JSON configuration file in the temp directory for testing purposes.
			tempDir := os.TempDir()
			bpConfigPath = filepath.Join(tempDir, "mock.json")
			tempFile, err := os.Create(bpConfigPath)
			Expect(err).To(BeNil())

			testData := config.BackplaneConfiguration{
				URL:             backplaneAPIURI,
				ProxyURL:        new(string),
				PagerDutyAPIKey: falsePagerDutyAPITkn,
			}

			// Marshal the testData into JSON format and write it to tempFile.
			pdTestData := testData
			pdTestData.PagerDutyAPIKey = ""
			jsonData, err := json.Marshal(pdTestData)
			Expect(err).To(BeNil())
			_, err = tempFile.Write(jsonData)
			Expect(err).To(BeNil())

			os.Setenv("BACKPLANE_CONFIG", bpConfigPath)

			err = runLogin(nil, nil)

			Expect(err.Error()).Should(ContainSubstring("please make sure the PD API Key is configured correctly"))
		})

		It("should fail to find a non existent PD Incident and return HTTP status code 404 when the requested resource is not found", func() {
			loginType = LoginTypePagerduty
			args.pd = falsePagerDutyIncidentID

			err := utils.CreateTempKubeConfig(nil)
			Expect(err).To(BeNil())

			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()

			// Create a temporary JSON configuration file in the temp directory for testing purposes.
			tempDir := os.TempDir()
			bpConfigPath = filepath.Join(tempDir, "mock.json")
			tempFile, err := os.Create(bpConfigPath)
			Expect(err).To(BeNil())

			testData := config.BackplaneConfiguration{
				URL:             backplaneAPIURI,
				ProxyURL:        new(string),
				PagerDutyAPIKey: truePagerDutyAPITkn,
			}

			// Marshal the testData into JSON format and write it to tempFile.
			jsonData, err := json.Marshal(testData)
			Expect(err).To(BeNil())
			_, err = tempFile.Write(jsonData)
			Expect(err).To(BeNil())

			os.Setenv("BACKPLANE_CONFIG", bpConfigPath)

			err = runLogin(nil, nil)

			Expect(err.Error()).Should(ContainSubstring("status code 404"))
		})

	})

	Context("check GetRestConfigAsUser", func() {
		It("check config creation with username and without elevationReasons", func() {
			mockOcmInterface.EXPECT().GetClusterInfoByID(testClusterID).Return(mockCluster, nil)
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(testClusterID)).Return(fakeResp, nil)

			username := "test-user"

			config, err := GetRestConfigAsUser(backplaneConfiguration, testClusterID, username)
			Expect(err).To(BeNil())
			Expect(config.Impersonate.UserName).To(Equal(username))
			Expect(len(config.Impersonate.Extra["reason"])).To(Equal(0))

		})

		It("check config creation with username and elevationReasons", func() {
			mockOcmInterface.EXPECT().GetClusterInfoByID(testClusterID).Return(mockCluster, nil)
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(testClusterID)).Return(fakeResp, nil)

			username := "test-user"
			elevationReasons := []string{"reason1", "reason2"}

			config, err := GetRestConfigAsUser(backplaneConfiguration, testClusterID, username, elevationReasons...)
			Expect(err).To(BeNil())
			Expect(config.Impersonate.UserName).To(Equal(username))
			Expect(config.Impersonate.Extra["reason"][0]).To(Equal(elevationReasons[0]))
			Expect(config.Impersonate.Extra["reason"][1]).To(Equal(elevationReasons[1]))

		})

	})

	Context("check JIRA OHSS login", func() {
		var (
			testOHSSID  string
			testIssue   jira.Issue
			issueFields *jira.IssueFields
		)
		BeforeEach(func() {
			mockIssueService = jiraMock.NewMockIssueServiceInterface(mockCtrl)
			ohssService = jiraClient.NewOHSSService(mockIssueService)
			testOHSSID = "OHSS-1000"
		})

		It("should login to ohss card cluster", func() {

			loginType = LoginTypeJira
			args.ohss = testOHSSID
			err := utils.CreateTempKubeConfig(nil)
			args.kubeConfigPath = ""
			Expect(err).To(BeNil())
			issueFields = &jira.IssueFields{
				Project:  jira.Project{Key: jiraClient.JiraOHSSProjectKey},
				Unknowns: tcontainer.MarshalMap{jiraClient.CustomFieldClusterID: testClusterID},
			}
			testIssue = jira.Issue{ID: testOHSSID, Fields: issueFields}
			globalOpts.ProxyURL = "https://squid.myproxy.com"
			mockIssueService.EXPECT().Get(testOHSSID, nil).Return(&testIssue, nil, nil).Times(1)
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockClientUtil.EXPECT().SetClientProxyURL(globalOpts.ProxyURL).Return(nil)
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(testClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(testClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(testClusterID)).Return(fakeResp, nil)

			err = runLogin(nil, nil)

			Expect(err).To(BeNil())
		})

		It("should failed missing cluster id ohss cards", func() {

			loginType = LoginTypeJira
			args.ohss = testOHSSID

			issueFields = &jira.IssueFields{
				Project: jira.Project{Key: jiraClient.JiraOHSSProjectKey},
			}
			testIssue = jira.Issue{ID: testOHSSID, Fields: issueFields}
			mockIssueService.EXPECT().Get(testOHSSID, nil).Return(&testIssue, nil, nil).Times(1)
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()

			err := runLogin(nil, nil)

			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal("clusterID cannot be detected for JIRA issue:OHSS-1000"))
		})
	})

	Context("RunLogin specific error paths", func() {
		var tempConfigFile *os.File
		var err error

		BeforeEach(func() {
			// Common setup for this context if any
		})

		AfterEach(func() {
			if tempConfigFile != nil {
				os.Remove(tempConfigFile.Name())
				tempConfigFile = nil
			}
			os.Unsetenv("BACKPLANE_CONFIG")
		})

		It("should fail if GetBackplaneConfiguration returns an error", func() {
			// Create a temporary invalid config file
			tempConfigFile, err = os.CreateTemp("", "invalid-config-*.json")
			Expect(err).NotTo(HaveOccurred())
			_, err = tempConfigFile.WriteString("{invalid_json_data:}")
			Expect(err).NotTo(HaveOccurred())
			err = tempConfigFile.Close()
			Expect(err).NotTo(HaveOccurred())

			os.Setenv("BACKPLANE_CONFIG", tempConfigFile.Name())

			// Call runLogin - expect an error due to invalid config
			// Provide minimal valid arguments to runLogin if necessary,
			// though it should fail before using them.
			// Assuming LoginTypeClusterID for this test, with a dummy cluster ID.
			loginType = LoginTypeClusterID
			err = runLogin(nil, []string{"dummyClusterID"})

			Expect(err).To(HaveOccurred())
			// The error message might vary depending on the JSON parser,
			// but it should indicate a config loading/parsing issue.
			// Example: "unable to load config file" or similar from viper/fsnotify
			// For now, just check that an error occurred.
			// A more specific check might be "error unmarshalling JSON" or similar based on actual error from config pkg.
			// Based on config.LoadOrNew, it uses viper, so error might be from viper.
			Expect(err.Error()).Should(Or(ContainSubstring("unmarshal errors"), ContainSubstring("Unable to load backplane config")))
		})

		Context("when loginType is LoginTypePagerduty", func() {
			BeforeEach(func() {
				loginType = LoginTypePagerduty
				args.pd = "dummyIncidentID" // A dummy incident ID for these tests
			})

			It("should fail if PagerDutyAPIKey is empty in bpConfig", func() {
				// Create a temporary valid config file with empty PagerDutyAPIKey
				tempConfigFile, err = os.CreateTemp("", "valid-config-empty-pdkey-*.json")
				Expect(err).NotTo(HaveOccurred())

				validConfigWithEmptyPDKey := config.BackplaneConfiguration{
					URL:             "https://example.com", // Need some valid URL
					PagerDutyAPIKey: "",                    // Empty key
				}
				configData, _ := json.Marshal(validConfigWithEmptyPDKey)
				_, err = tempConfigFile.Write(configData)
				Expect(err).NotTo(HaveOccurred())
				err = tempConfigFile.Close()
				Expect(err).NotTo(HaveOccurred())
				os.Setenv("BACKPLANE_CONFIG", tempConfigFile.Name())

				// Mock OCM environment as it's called before PagerDuty logic sometimes (e.g. in error path of GetTargetCluster)
				// However, GetBackplaneConfiguration is called first.
				// If GetTargetCluster is reached, it might need mocks.
				// In this specific path, runLogin should return before GetTargetCluster if PD key is empty.
				mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()


				err = runLogin(nil, []string{}) // No argv needed as PD incident ID is from args.pd
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("please make sure the PD API Key is configured correctly in the config file"))
			})

			It("should fail if GetClusterInfoFromIncident fails (via URL parsing path with non-existent ID)", func() {
				args.pd = "https://sometenant.pagerduty.com/incidents/" + falsePagerDutyIncidentID // falsePagerDutyIncidentID is known to cause 404

				// Create a temporary valid config file with the true PagerDutyAPIKey
				tempConfigFile, err = os.CreateTemp("", "valid-config-true-pdkey-*.json")
				Expect(err).NotTo(HaveOccurred())

				validConfigWithTruePDKey := config.BackplaneConfiguration{
					URL:             "https://example.com",
					PagerDutyAPIKey: truePagerDutyAPITkn, // True test token
				}
				configData, _ := json.Marshal(validConfigWithTruePDKey)
				_, err = tempConfigFile.Write(configData)
				Expect(err).NotTo(HaveOccurred())
				err = tempConfigFile.Close()
				Expect(err).NotTo(HaveOccurred())
				os.Setenv("BACKPLANE_CONFIG", tempConfigFile.Name())

				mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()

				err = runLogin(nil, []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).Should(ContainSubstring("status code 404")) // PagerDuty API should return 404 for non-existent incident
			})
		})

		Context("when loginType is LoginTypeJira", func() {
			var expectedJiraErr error

			BeforeEach(func() {
				loginType = LoginTypeJira
				args.ohss = "OHSS-DUMMY"
				expectedJiraErr = errors.New("jira get issue failed")

				// Ensure bpConfig is valid and doesn't cause early exit
				// This can be done by setting up a minimal valid temp config file once for the parent context
				// or ensuring default GetBackplaneConfiguration works if not overridden by specific tests.
				// For simplicity, let's ensure a valid config for these Jira tests.
				tempConfigFile, err = os.CreateTemp("", "valid-config-jira-*.json")
				Expect(err).NotTo(HaveOccurred())
				validConfig := config.BackplaneConfiguration{URL: "https://example.com"}
				configData, _ := json.Marshal(validConfig)
				_, err = tempConfigFile.Write(configData)
				Expect(err).NotTo(HaveOccurred())
				err = tempConfigFile.Close()
				Expect(err).NotTo(HaveOccurred())
				os.Setenv("BACKPLANE_CONFIG", tempConfigFile.Name())

				// Setup mockIssueService for JIRA tests (defined in the outer Describe's BeforeEach)
				// If mockIssueService is not initialized, this will panic.
				// It's initialized in the main Describe's BeforeEach.
				// Re-assign ohssService to use the mocked one for each test.
				mockIssueService = jiraMock.NewMockIssueServiceInterface(mockCtrl)
				ohssService = jiraClient.NewOHSSService(mockIssueService)

				mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			})

			It("should fail if ohssService.GetIssue returns an error", func() {
				mockIssueService.EXPECT().Get(args.ohss, nil).Return(nil, nil, expectedJiraErr).Times(1)

				err = runLogin(nil, []string{}) // No argv needed as JIRA ID is from args.ohss
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expectedJiraErr.Error()))
			})

			It("should fail if JIRA issue is retrieved but ClusterID is empty", func() {
				emptyClusterIDIssue := jira.Issue{
					ID: args.ohss,
					Fields: &jira.IssueFields{
						Project:  jira.Project{Key: jiraClient.JiraOHSSProjectKey},
						Unknowns: tcontainer.MarshalMap{}, // No ClusterID custom field
					},
				}
				// ohssService.GetIssue returns an OHSSIssue, not a jira.Issue directly.
				// The conversion logic is within ohssService.GetIssue.
				// So we mock Get to return a jira.Issue that will result in an empty ClusterID in the OHSSIssue.
				// The actual OHSSIssue conversion happens inside jira.GetIssue -> mapToDO(issue)
				// mapToDO will set ClusterID from issue.Fields.Unknowns[CustomFieldClusterID]
				mockIssueService.EXPECT().Get(args.ohss, nil).Return(&emptyClusterIDIssue, nil, nil).Times(1)

				err = runLogin(nil, []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(fmt.Sprintf("clusterID cannot be detected for JIRA issue:%s", args.ohss)))
			})
		})

		Context("when loginType is LoginTypeExistingKubeConfig", func() {
			BeforeEach(func() {
				loginType = LoginTypeExistingKubeConfig

				// Ensure bpConfig is valid
				tempConfigFile, err = os.CreateTemp("", "valid-config-kubeconfig-*.json")
				Expect(err).NotTo(HaveOccurred())
				validConfig := config.BackplaneConfiguration{URL: "https://example.com"}
				configData, _ := json.Marshal(validConfig)
				_, err = tempConfigFile.Write(configData)
				Expect(err).NotTo(HaveOccurred())
				err = tempConfigFile.Close()
				Expect(err).NotTo(HaveOccurred())
				os.Setenv("BACKPLANE_CONFIG", tempConfigFile.Name())

				// Ensure Kubeconfig is empty/invalid for this test,
				// which is typically handled by the main Describe's BeforeEach.
				// clientcmd.ModifyConfig(clientcmd.NewDefaultPathOptions(), api.Config{}, true)
				// This should cause GetBackplaneClusterFromConfig to fail.
				mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()

			})

			It("should fail if GetBackplaneClusterFromConfig returns an error", func() {
				// With an empty kubeconfig, GetBackplaneClusterFromConfig should fail.
				// The error is usually "current context is not set" or similar.
				err = runLogin(nil, []string{}) // No argv needed for this login type
				Expect(err).To(HaveOccurred())
				// Example error from utils.GetBackplaneClusterFromConfig() with empty config:
				// "config view is nil" or "current context is not set"
				// Let's check for a substring that's likely.
				Expect(err.Error()).Should(MatchRegexp("(?i)kubeconfig|context|cluster information"))
			})
		})

		It("should fail if ocm.GetTargetCluster returns an error", func() {
			loginType = LoginTypeClusterID // Use a simple login type for this
			clusterKey := "dummyClusterIDForTargetClusterFail"
			expectedErr := errors.New("get target cluster failed")

			// Ensure bpConfig is valid
			tempConfigFile, err = os.CreateTemp("", "valid-config-*.json")
			Expect(err).NotTo(HaveOccurred())
			validConfig := config.BackplaneConfiguration{URL: "https://example.com"}
			configData, _ := json.Marshal(validConfig)
			_, err = tempConfigFile.Write(configData)
			Expect(err).NotTo(HaveOccurred())
			err = tempConfigFile.Close()
			Expect(err).NotTo(HaveOccurred())
			os.Setenv("BACKPLANE_CONFIG", tempConfigFile.Name())

			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes() // May be called by proxy setup or other preceding logic
			mockOcmInterface.EXPECT().GetTargetCluster(clusterKey).Return("", "", expectedErr).Times(1)

			err = runLogin(nil, []string{clusterKey})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(expectedErr.Error()))
		})

		Context("when globalOpts.Manager is true", func() {
			var originalClusterID string
			var originalClusterName string

			BeforeEach(func() {
				globalOpts.Manager = true
				loginType = LoginTypeClusterID
				originalClusterID = "original-cluster-id"
				originalClusterName = "original-cluster-name"

				// Ensure bpConfig is valid
				tempConfigFile, err = os.CreateTemp("", "valid-config-manager-*.json")
				Expect(err).NotTo(HaveOccurred())
				validConfig := config.BackplaneConfiguration{URL: "https_example.com"}
				configData, _ := json.Marshal(validConfig)
				_, err = tempConfigFile.Write(configData)
				Expect(err).NotTo(HaveOccurred())
				err = tempConfigFile.Close()
				Expect(err).NotTo(HaveOccurred())
				os.Setenv("BACKPLANE_CONFIG", tempConfigFile.Name())

				// Common successful GetTargetCluster mock
				mockOcmInterface.EXPECT().GetTargetCluster(gomock.Any()).Return(originalClusterID, originalClusterName, nil).AnyTimes()
				// Common GetOCMEnvironment mock
				mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
				// Mock IsClusterHibernating for the original cluster to pass DisplayClusterInfo if enabled
				mockOcmInterface.EXPECT().IsClusterHibernating(originalClusterID).Return(false, nil).AnyTimes()

				// Default mocks for PrintClusterInfo if args.clusterInfo or bpConfig.DisplayClusterInfo is true
				// These might be called before GetManagingCluster fails, depending on config.
				// Add lenient mocks for them.
				if args.clusterInfo || backplaneConfiguration.DisplayClusterInfo {
					mockOcmInterface.EXPECT().GetClusterInfoByID(originalClusterID).Return(mockCluster, nil).AnyTimes()
					mockOcmInterface.EXPECT().SetupOCMConnection().Return(nil, nil).AnyTimes()
					mockOcmInterface.EXPECT().IsClusterAccessProtectionEnabled(gomock.Any(), originalClusterID).Return(false, nil).AnyTimes()
				}
			})

			AfterEach(func() {
				globalOpts.Manager = false
			})

			It("should fail if ocm.GetManagingCluster returns an error", func() {
				expectedErr := errors.New("get managing cluster failed")
				mockOcmInterface.EXPECT().GetManagingCluster(originalClusterID).Return("", "", false, expectedErr).Times(1)

				err = runLogin(nil, []string{originalClusterID}) // Pass originalClusterID as argv
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expectedErr.Error()))
			})

			It("should fail if listNamespaces returns an error (e.g. GetClusterInfoByID fails)", func() {
				managingClusterID := "managing-cluster-id"
				managingClusterName := "managing-cluster-name"
				isHostedControlPlane := false // or true, doesn't fundamentally change this error path

				// GetManagingCluster must succeed for listNamespaces to be called
				mockOcmInterface.EXPECT().GetManagingCluster(originalClusterID).Return(managingClusterID, managingClusterName, isHostedControlPlane, nil).Times(1)

				// listNamespaces calls GetOCMEnvironment first
				mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).Times(1) // ocmEnv is from global test setup

				// Then listNamespaces calls GetClusterInfoByID with the *original* cluster ID
				expectedListNamespacesErr := errors.New("listNamespaces GetClusterInfoByID failed")
				mockOcmInterface.EXPECT().GetClusterInfoByID(originalClusterID).Return(nil, expectedListNamespacesErr).Times(1)


				err = runLogin(nil, []string{originalClusterID})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expectedListNamespacesErr.Error()))
			})
		})

		Context("when globalOpts.Service is true", func() {
			var initialTargetClusterID string
			var initialTargetClusterName string

			BeforeEach(func() {
				globalOpts.Service = true
				loginType = LoginTypeClusterID
				initialTargetClusterID = "initial-target-id"
				initialTargetClusterName = "initial-target-name"

				// Ensure bpConfig is valid
				tempConfigFile, err = os.CreateTemp("", "valid-config-service-*.json")
				Expect(err).NotTo(HaveOccurred())
				validConfig := config.BackplaneConfiguration{URL: "https_example.com"}
				configData, _ := json.Marshal(validConfig)
				_, err = tempConfigFile.Write(configData)
				Expect(err).NotTo(HaveOccurred())
				err = tempConfigFile.Close()
				Expect(err).NotTo(HaveOccurred())
				os.Setenv("BACKPLANE_CONFIG", tempConfigFile.Name())

				// Common successful GetTargetCluster mock
				mockOcmInterface.EXPECT().GetTargetCluster(gomock.Any()).Return(initialTargetClusterID, initialTargetClusterName, nil).AnyTimes()
				// Common GetOCMEnvironment mock
				mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
				// Mock IsClusterHibernating for the original cluster
				mockOcmInterface.EXPECT().IsClusterHibernating(initialTargetClusterID).Return(false, nil).AnyTimes()

				// Default mocks for PrintClusterInfo if enabled (called with initialTargetClusterID)
				if args.clusterInfo || backplaneConfiguration.DisplayClusterInfo {
					mockOcmInterface.EXPECT().GetClusterInfoByID(initialTargetClusterID).Return(mockCluster, nil).AnyTimes()
					mockOcmInterface.EXPECT().SetupOCMConnection().Return(nil, nil).AnyTimes()
					mockOcmInterface.EXPECT().IsClusterAccessProtectionEnabled(gomock.Any(), initialTargetClusterID).Return(false, nil).AnyTimes()
				}
			})

			AfterEach(func() {
				globalOpts.Service = false
			})

			It("should fail if the first GetManagingCluster (for HCP check) returns an error", func() {
				expectedErr := errors.New("get managing cluster for hcp check failed")
				mockOcmInterface.EXPECT().GetManagingCluster(initialTargetClusterID).Return("", "", false, expectedErr).Times(1)

				err = runLogin(nil, []string{initialTargetClusterID})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expectedErr.Error()))
			})

			It("should fail if ocm.GetServiceCluster returns an error", func() {
				// First GetManagingCluster (for HCP check) must succeed
				managingClusterName := "managing-cluster-for-hcp-check"
				isHostedControlPlane := true // Assume true to proceed to GetServiceCluster
				mockOcmInterface.EXPECT().GetManagingCluster(initialTargetClusterID).Return(managingClusterID, managingClusterName, isHostedControlPlane, nil).Times(1)

				expectedErr := errors.New("get service cluster failed")
				mockOcmInterface.EXPECT().GetServiceCluster(initialTargetClusterID).Return("", "", expectedErr).Times(1)

				err = runLogin(nil, []string{initialTargetClusterID})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expectedErr.Error()))
			})

			It("should fail if not a hosted control plane cluster", func() {
				// First GetManagingCluster (for HCP check) must succeed and return isHostedControlPlane = false
				managingClusterName := "managing-cluster-for-hcp-check"
				isHostedControlPlane := false // This is the key for this test
				mockOcmInterface.EXPECT().GetManagingCluster(initialTargetClusterID).Return(managingClusterID, managingClusterName, isHostedControlPlane, nil).Times(1)

				// GetServiceCluster must also succeed for the HCP check to be the point of failure
				serviceClusterID := "service-id"
				serviceClusterName := "service-name"
				mockOcmInterface.EXPECT().GetServiceCluster(initialTargetClusterID).Return(serviceClusterID, serviceClusterName, nil).Times(1)

				err = runLogin(nil, []string{initialTargetClusterID})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("manifestworks are only available for hosted control plane clusters"))
			})
		})

		It("should fail if login.SaveKubeConfig returns an error", func() {
			loginType = LoginTypeClusterID
			targetClusterKey := "targetClusterForSaveKubeConfigFail"
			targetClusterID := "targetClusterID-actual"
			targetClusterName := "targetClusterName-actual"
			dummyProxyURL := "https://proxy.example.com"

			// Setup for SaveKubeConfig to fail: make kubeConfigPath a file
			tempWriteTestFile, err := os.CreateTemp("", "readonly-file-")
			Expect(err).NotTo(HaveOccurred())
			tempWriteTestFile.Close() // Close it so it's just a file
			defer os.Remove(tempWriteTestFile.Name())

			args.multiCluster = true // Enable multi-cluster to use kubeConfigPath logic in SaveKubeConfig
			args.kubeConfigPath = tempWriteTestFile.Name() // Set path to be a file

			// Ensure bpConfig is valid
			// Using the global tempConfigFile from the parent Context
			bpFile, err := os.CreateTemp("", "valid-config-savekubeconfig-*.json")
			Expect(err).NotTo(HaveOccurred())
			defer os.Remove(bpFile.Name())
			validConfig := config.BackplaneConfiguration{URL: "https_example.com"}
			configData, _ := json.Marshal(validConfig)
			_, err = bpFile.Write(configData)
			Expect(err).NotTo(HaveOccurred())
			err = bpFile.Close()
			Expect(err).NotTo(HaveOccurred())
			os.Setenv("BACKPLANE_CONFIG", bpFile.Name())


			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(targetClusterKey).Return(targetClusterID, targetClusterName, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(targetClusterID).Return(false, nil)
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil) // testToken from global test setup

			// Mock for doLogin
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(gomock.Any(), testToken).Return(mockClient, nil)
			// Prepare a response for client.LoginCluster that doLogin can parse
			loginRespBody := fmt.Sprintf(`{"proxy_uri": "%s"}`, strings.Replace(dummyProxyURL, "https_example.com", "", 1))
			fakeRespForDoLogin := &http.Response{
				Body:       MakeIoReader(loginRespBody),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				StatusCode: http.StatusOK,
			}
			mockClient.EXPECT().LoginCluster(gomock.Any(), targetClusterID).Return(fakeRespForDoLogin, nil)


			err = runLogin(nil, []string{targetClusterKey})
			Expect(err).To(HaveOccurred())
			// The error from SaveKubeConfig when path is a file is typically like "mkdir <path>: not a directory"
			// or "failed to save kubeconfig for cluster..."
			Expect(err.Error()).Should(Or(
				ContainSubstring("not a directory"),
				ContainSubstring("failed to save kubeconfig"),
				ContainSubstring("is a file not a directory"), // error from pkg/login/kubeconfig.go
			))

			// Cleanup args
			args.multiCluster = false
			args.kubeConfigPath = ""
		})
	})
})
