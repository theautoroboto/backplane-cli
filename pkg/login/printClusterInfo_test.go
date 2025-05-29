package login

import (
	"bytes"
	"fmt"
	"log"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/backplane-cli/pkg/ocm"
	ocmMock "github.com/openshift/backplane-cli/pkg/ocm/mocks"

	ocmsdk "github.com/openshift-online/ocm-sdk-go"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
)

var _ = Describe("PrintClusterInfo", func() {
	var (
		clusterID        string
		buf              *bytes.Buffer
		mockOcmInterface *ocmMock.MockOCMInterface
		mockCtrl         *gomock.Controller
		oldStdout        *os.File
		r, w             *os.File
		ocmConnection    *ocmsdk.Connection
		clusterInfo      *cmv1.Cluster
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockOcmInterface = ocmMock.NewMockOCMInterface(mockCtrl)
		ocmConnection = nil
		ocm.DefaultOCMInterface = mockOcmInterface

		clusterID = "test-cluster-id"
		buf = new(bytes.Buffer)
		log.SetOutput(buf)

		// Redirect standard output to the buffer
		oldStdout = os.Stdout
		r, w, _ = os.Pipe()
		os.Stdout = w
	})

	AfterEach(func() {
		// Reset the ocm.DefaultOCMInterface to avoid side effects in other tests
		ocm.DefaultOCMInterface = nil
	})

	Context("Cluster protection status", func() {
		BeforeEach(func() {

			clusterInfo, _ = cmv1.NewCluster().
				ID(clusterID).
				Name("Test Cluster").
				CloudProvider(cmv1.NewCloudProvider().ID("aws")).
				State(cmv1.ClusterState("ready")).
				Region(cmv1.NewCloudRegion().ID("us-east-1")).
				Hypershift(cmv1.NewHypershift().Enabled(false)).
				OpenshiftVersion("4.14.8").
				Status(cmv1.NewClusterStatus().LimitedSupportReasonCount(0)).
				Build()

			mockOcmInterface.EXPECT().GetClusterInfoByID(clusterID).Return(clusterInfo, nil).AnyTimes()
			mockOcmInterface.EXPECT().SetupOCMConnection().Return(ocmConnection, nil).AnyTimes()
		})

		It("should print cluster information with access protection disabled", func() {
			mockOcmInterface.EXPECT().IsClusterAccessProtectionEnabled(ocmConnection, clusterID).Return(false, nil).AnyTimes()

			err := PrintClusterInfo(clusterID)
			Expect(err).To(BeNil())

			// Capture the output
			w.Close()
			os.Stdout = oldStdout
			_, _ = buf.ReadFrom(r)

			output := buf.String()
			Expect(output).To(ContainSubstring(fmt.Sprintf("Cluster ID:               %s\n", clusterID)))
			Expect(output).To(ContainSubstring("Cluster Name:             Test Cluster\n"))
			Expect(output).To(ContainSubstring("Cluster Status:           ready\n"))
			Expect(output).To(ContainSubstring("Cluster Region:           us-east-1\n"))
			Expect(output).To(ContainSubstring("Cluster Provider:         aws\n"))
			Expect(output).To(ContainSubstring("Hypershift Enabled:       false\n"))
			Expect(output).To(ContainSubstring("Version:                  4.14.8\n"))
			Expect(output).To(ContainSubstring("Limited Support Status:   Fully Supported\n"))
			Expect(output).To(ContainSubstring("Access Protection:        Disabled\n"))
		})

		It("should print cluster information with access protection enabled", func() {
			mockOcmInterface.EXPECT().IsClusterAccessProtectionEnabled(ocmConnection, clusterID).Return(true, nil).AnyTimes()

			err := PrintClusterInfo(clusterID)
			Expect(err).To(BeNil())

			// Capture the output
			w.Close()
			os.Stdout = oldStdout
			_, _ = buf.ReadFrom(r)

			output := buf.String()
			Expect(output).To(ContainSubstring(fmt.Sprintf("Cluster ID:               %s\n", clusterID)))
			Expect(output).To(ContainSubstring("Cluster Name:             Test Cluster\n"))
			Expect(output).To(ContainSubstring("Cluster Status:           ready\n"))
			Expect(output).To(ContainSubstring("Cluster Region:           us-east-1\n"))
			Expect(output).To(ContainSubstring("Cluster Provider:         aws\n"))
			Expect(output).To(ContainSubstring("Hypershift Enabled:       false\n"))
			Expect(output).To(ContainSubstring("Version:                  4.14.8\n"))
			Expect(output).To(ContainSubstring("Limited Support Status:   Fully Supported\n"))
			Expect(output).To(ContainSubstring("Access Protection:        Enabled\n"))
		})
	})

	Context("Limited Support set to 0", func() {
		BeforeEach(func() {

			clusterInfo, _ = cmv1.NewCluster().
				ID(clusterID).
				Name("Test Cluster").
				CloudProvider(cmv1.NewCloudProvider().ID("aws")).
				State(cmv1.ClusterState("ready")).
				Region(cmv1.NewCloudRegion().ID("us-east-1")).
				Hypershift(cmv1.NewHypershift().Enabled(false)).
				OpenshiftVersion("4.14.8").
				Status(cmv1.NewClusterStatus().LimitedSupportReasonCount(0)).
				Build()

			mockOcmInterface.EXPECT().GetClusterInfoByID(clusterID).Return(clusterInfo, nil).AnyTimes()
			mockOcmInterface.EXPECT().SetupOCMConnection().Return(ocmConnection, nil).AnyTimes()
		})

		It("should print if cluster is Fully Supported", func() {

			mockOcmInterface.EXPECT().IsClusterAccessProtectionEnabled(ocmConnection, clusterID).Return(true, nil).AnyTimes()

			err := PrintClusterInfo(clusterID)
			Expect(err).To(BeNil())

			// Capture the output
			w.Close()
			os.Stdout = oldStdout
			_, _ = buf.ReadFrom(r)

			output := buf.String()
			Expect(output).To(ContainSubstring(fmt.Sprintf("Cluster ID:               %s\n", clusterID)))
			Expect(output).To(ContainSubstring("Cluster Name:             Test Cluster\n"))
			Expect(output).To(ContainSubstring("Cluster Status:           ready\n"))
			Expect(output).To(ContainSubstring("Cluster Region:           us-east-1\n"))
			Expect(output).To(ContainSubstring("Cluster Provider:         aws\n"))
			Expect(output).To(ContainSubstring("Hypershift Enabled:       false\n"))
			Expect(output).To(ContainSubstring("Version:                  4.14.8\n"))
			Expect(output).To(ContainSubstring("Limited Support Status:   Fully Supported\n"))
			Expect(output).To(ContainSubstring("Access Protection:        Enabled\n"))
		})

	})

	Context("Limited Support set to 1", func() {
		BeforeEach(func() {

			clusterInfo, _ = cmv1.NewCluster().
				ID(clusterID).
				Name("Test Cluster").
				CloudProvider(cmv1.NewCloudProvider().ID("aws")).
				State(cmv1.ClusterState("ready")).
				Region(cmv1.NewCloudRegion().ID("us-east-1")).
				Hypershift(cmv1.NewHypershift().Enabled(false)).
				OpenshiftVersion("4.14.8").
				Status(cmv1.NewClusterStatus().LimitedSupportReasonCount(1)).
				Build()

			mockOcmInterface.EXPECT().GetClusterInfoByID(clusterID).Return(clusterInfo, nil).AnyTimes()
			mockOcmInterface.EXPECT().SetupOCMConnection().Return(ocmConnection, nil).AnyTimes()
		})

		It("should print if cluster is Limited Support", func() {
			mockOcmInterface.EXPECT().IsClusterAccessProtectionEnabled(ocmConnection, clusterID).Return(true, nil).AnyTimes()
			err := PrintClusterInfo(clusterID)
			Expect(err).To(BeNil())

			// Capture the output
			w.Close()
			os.Stdout = oldStdout
			_, _ = buf.ReadFrom(r)

			output := buf.String()
			Expect(output).To(ContainSubstring(fmt.Sprintf("Cluster ID:               %s\n", clusterID)))
			Expect(output).To(ContainSubstring("Cluster Name:             Test Cluster\n"))
			Expect(output).To(ContainSubstring("Cluster Status:           ready\n"))
			Expect(output).To(ContainSubstring("Cluster Region:           us-east-1\n"))
			Expect(output).To(ContainSubstring("Cluster Provider:         aws\n"))
			Expect(output).To(ContainSubstring("Hypershift Enabled:       false\n"))
			Expect(output).To(ContainSubstring("Version:                  4.14.8\n"))
			Expect(output).To(ContainSubstring("Limited Support Status:   Limited Support\n"))
			Expect(output).To(ContainSubstring("Access Protection:        Enabled\n"))
		})
	})

	It("should return an error if GetClusterInfoByID fails", func() {
		expectedErr := fmt.Errorf("failed to get cluster info")
		mockOcmInterface.EXPECT().GetClusterInfoByID(clusterID).Return(nil, expectedErr)

		err := PrintClusterInfo(clusterID)
		Expect(err).To(MatchError(expectedErr))

		// Capture the output
		w.Close()
		os.Stdout = oldStdout
		_, _ = buf.ReadFrom(r)

		output := buf.String()
		Expect(output).To(BeEmpty())
	})

	Context("GetAccessProtectionStatus specific tests", func() {
		BeforeEach(func() {
			// Common setup for these tests: GetClusterInfoByID is expected to succeed.
			clusterInfo, _ = cmv1.NewCluster().
				ID(clusterID).
				Name("Test Cluster").
				CloudProvider(cmv1.NewCloudProvider().ID("aws")).
				State(cmv1.ClusterState("ready")).
				Region(cmv1.NewCloudRegion().ID("us-east-1")).
				Hypershift(cmv1.NewHypershift().Enabled(false)).
				OpenshiftVersion("4.14.8").
				Status(cmv1.NewClusterStatus().LimitedSupportReasonCount(0)).
				Build()
			// We don't care about the limited support status call for these specific tests,
			// but GetClusterInfoByID will be called by it first in PrintClusterInfo.
			mockOcmInterface.EXPECT().GetClusterInfoByID(clusterID).Return(clusterInfo, nil).AnyTimes()
		})

		It("should print and return error if SetupOCMConnection fails for GetAccessProtectionStatus", func() {
			expectedErr := fmt.Errorf("failed to setup ocm connection")
			mockOcmInterface.EXPECT().SetupOCMConnection().Return(nil, expectedErr)

			// Call PrintClusterInfo, which internally calls GetAccessProtectionStatus
			err := PrintClusterInfo(clusterID)
			Expect(err).To(BeNil()) // PrintClusterInfo itself doesn't return this error

			// Capture the output
			w.Close()
			os.Stdout = oldStdout
			_, _ = buf.ReadFrom(r)
			output := buf.String()

			Expect(output).To(ContainSubstring("Error setting up OCM connection: " + expectedErr.Error()))
			// Also check that the "Access Protection:" line reflects the error indirectly
			// The function GetAccessProtectionStatus prints the status line itself.
			// As the error is logged by logrus, we check the log buffer instead of stdout for the exact error message.
			// For stdout, we check the user-facing message.
			// Reset log output to capture it
			logOutput := new(bytes.Buffer)
			log.SetOutput(logOutput)
			// Re-run the function whose output we want to check more directly if possible,
			// or check side effects. Here, GetAccessProtectionStatus is called by PrintClusterInfo.
			// The design of GetAccessProtectionStatus is to print and return a string.
			// Let's re-evaluate how to best test this.
			// The function GetAccessProtectionStatus prints to stdout AND returns a string.
			// The PrintClusterInfo function calls GetAccessProtectionStatus but doesn't use its return value directly for error handling.
			// The error from SetupOCMConnection is logged by logrus within GetAccessProtectionStatus.

			// Let's check the log output for the specific error.
			// We need to re-run the specific part or ensure logs are captured during the PrintClusterInfo call.
			// The initial buf captures os.Stdout. For logrus output, we need to set its output.
			// The test setup already redirects log.SetOutput(buf)
			// So, the logrus error message should be in 'output'.
			Expect(output).To(ContainSubstring("Error setting up OCM connection: failed to setup ocm connection"))
			// If SetupOCMConnection fails, GetAccessProtectionStatus returns early, before printing the "Access Protection:" line.
			Expect(output).NotTo(ContainSubstring("Access Protection:"))
		})

		It("should print and return error if IsClusterAccessProtectionEnabled fails for GetAccessProtectionStatus", func() {
			expectedErr := fmt.Errorf("failed to check access protection status")
			// Ensure SetupOCMConnection is successful
			mockOcmInterface.EXPECT().SetupOCMConnection().Return(ocmConnection, nil)
			mockOcmInterface.EXPECT().IsClusterAccessProtectionEnabled(ocmConnection, clusterID).Return(false, expectedErr)

			err := PrintClusterInfo(clusterID)
			Expect(err).To(BeNil()) // PrintClusterInfo itself doesn't return this error

			// Capture the output
			w.Close()
			os.Stdout = oldStdout
			_, _ = buf.ReadFrom(r)
			output := buf.String()

			// The function GetAccessProtectionStatus prints the error to fmt.Println
			Expect(output).To(ContainSubstring("Error retrieving access protection status: " + expectedErr.Error()))
			// And it returns before printing the "Access Protection:" status line.
			Expect(output).NotTo(ContainSubstring("Access Protection: Enabled"))
			Expect(output).NotTo(ContainSubstring("Access Protection: Disabled"))
		})

		It("should print Access Protection: Disabled in govcloud environment for GetAccessProtectionStatus", func() {
			viper.Set("govcloud", true)
			defer viper.Set("govcloud", false) // Cleanup viper setting

			// SetupOCMConnection is still expected to be called
			mockOcmInterface.EXPECT().SetupOCMConnection().Return(ocmConnection, nil)
			// IsClusterAccessProtectionEnabled should NOT be called in govcloud
			mockOcmInterface.EXPECT().IsClusterAccessProtectionEnabled(gomock.Any(), gomock.Any()).Times(0)

			err := PrintClusterInfo(clusterID)
			Expect(err).To(BeNil())

			// Capture the output
			w.Close()
			os.Stdout = oldStdout
			_, _ = buf.ReadFrom(r)
			output := buf.String()

			Expect(output).To(ContainSubstring("Access Protection:        Disabled"))
			// Ensure no error messages related to access protection are printed
			Expect(output).NotTo(ContainSubstring("Error retrieving access protection status:"))
			Expect(output).NotTo(ContainSubstring("Error setting up OCM connection:"))
		})
	})

	Context("GetLimitedSupportStatus specific tests", func() {
		It("should not print Limited Support Status if GetClusterInfoByID fails within GetLimitedSupportStatus", func() {
			expectedErr := fmt.Errorf("failed to get cluster info for limited support status")

			// Mock for the GetClusterInfoByID call at the beginning of PrintClusterInfo
			// This one must succeed for PrintClusterInfo to proceed.
			initialClusterInfo, _ := cmv1.NewCluster().
				ID(clusterID).
				Name("Test Cluster").
				CloudProvider(cmv1.NewCloudProvider().ID("aws")).
				State(cmv1.ClusterState("ready")).
				Region(cmv1.NewCloudRegion().ID("us-east-1")).
				Hypershift(cmv1.NewHypershift().Enabled(false)).
				OpenshiftVersion("4.14.8").
				Status(cmv1.NewClusterStatus().LimitedSupportReasonCount(0)). // Status doesn't matter here
				Build()
			mockOcmInterface.EXPECT().GetClusterInfoByID(clusterID).Return(initialClusterInfo, nil).Times(1)

			// Mock for the GetClusterInfoByID call within GetLimitedSupportStatus
			// This is the one we want to fail.
			mockOcmInterface.EXPECT().GetClusterInfoByID(clusterID).Return(nil, expectedErr).Times(1)

			// GetAccessProtectionStatus will also be called. We need to ensure its mocks are satisfied.
			// Assuming govcloud is false and no errors in its path for this specific test focus.
			viper.Set("govcloud", false) // Ensure consistent state
			mockOcmInterface.EXPECT().SetupOCMConnection().Return(ocmConnection, nil)
			mockOcmInterface.EXPECT().IsClusterAccessProtectionEnabled(ocmConnection, clusterID).Return(false, nil)


			err := PrintClusterInfo(clusterID)
			Expect(err).To(BeNil()) // PrintClusterInfo itself should not error out due to this internal issue

			w.Close()
			os.Stdout = oldStdout
			_, _ = buf.ReadFrom(r)
			output := buf.String()

			// Basic cluster info should still be printed from PrintClusterInfo's direct GetClusterInfoByID call
			Expect(output).To(ContainSubstring(fmt.Sprintf("Cluster ID:               %s\n", clusterID)))
			Expect(output).To(ContainSubstring("Cluster Name:             Test Cluster\n"))

			// The "Limited Support Status:" line should NOT be printed
			Expect(output).NotTo(ContainSubstring("Limited Support Status:"))

			// Access Protection status should be printed as normal
			Expect(output).To(ContainSubstring("Access Protection:        Disabled"))
		})
	})
})
