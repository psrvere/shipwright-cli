package build // nolint:revive

import (
	"strings"
	"testing"

	buildv1beta1 "github.com/shipwright-io/build/pkg/apis/build/v1beta1"
	shpfake "github.com/shipwright-io/build/pkg/client/clientset/versioned/fake"
	"github.com/shipwright-io/cli/pkg/shp/flags"
	"github.com/shipwright-io/cli/pkg/shp/params"
	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes/fake"
)

// newUploadCommandForTest builds an UploadCommand wired with a fresh cobra command and the standard
// BuildRun spec flags, mimicking uploadCmd().
func newUploadCommandForTest() *UploadCommand {
	ccmd := &cobra.Command{}
	u := &UploadCommand{
		cmd:          ccmd,
		buildRunSpec: flags.BuildRunSpecFromFlags(ccmd.Flags()),
	}
	flags.FollowFlag(ccmd.Flags(), &u.follow)
	buildRunNameFlag(ccmd.Flags(), &u.buildRunName)
	return u
}

func TestUploadValidateBuildRunNameConflicts(t *testing.T) {
	t.Run("buildrun-name with a spec flag is rejected", func(t *testing.T) {
		u := newUploadCommandForTest()
		if err := u.cmd.Flags().Set(flags.BuildrunNameFlag, "existing-br"); err != nil {
			t.Fatal(err)
		}
		if err := u.cmd.Flags().Set(flags.ServiceAccountNameFlag, "builder"); err != nil {
			t.Fatal(err)
		}

		err := u.checkBuildRunNameConflicts()
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		if !strings.Contains(err.Error(), flags.ServiceAccountNameFlag) {
			t.Errorf("error should mention the conflicting flag, got: %s", err.Error())
		}
	})

	t.Run("buildrun-name alone is accepted", func(t *testing.T) {
		u := newUploadCommandForTest()
		if err := u.cmd.Flags().Set(flags.BuildrunNameFlag, "existing-br"); err != nil {
			t.Fatal(err)
		}
		// buildref-name is set programmatically from the positional argument
		if err := u.cmd.Flags().Set(flags.BuildrefNameFlag, "my-build"); err != nil {
			t.Fatal(err)
		}

		if err := u.checkBuildRunNameConflicts(); err != nil {
			t.Errorf("expected no error, got: %s", err.Error())
		}
	})

	t.Run("no buildrun-name allows spec flags", func(t *testing.T) {
		u := newUploadCommandForTest()
		if err := u.cmd.Flags().Set(flags.ServiceAccountNameFlag, "builder"); err != nil {
			t.Fatal(err)
		}

		if err := u.checkBuildRunNameConflicts(); err != nil {
			t.Errorf("expected no error, got: %s", err.Error())
		}
	})
}

func TestUploadResolveBuildRunFetchesExisting(t *testing.T) {
	existing := &buildv1beta1.BuildRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceDefault,
			Name:      "existing-br",
		},
	}
	shpclientset := shpfake.NewSimpleClientset(existing)
	kclientset := fake.NewSimpleClientset()
	param := params.NewParamsForTest(kclientset, shpclientset, nil, genericclioptions.NewConfigFlags(true), metav1.NamespaceDefault, nil, nil)

	ioStreams, _, _, _ := genericclioptions.NewTestIOStreams()

	u := newUploadCommandForTest()
	u.ioStreams = &ioStreams
	u.buildRunName = "existing-br"

	br, err := u.resolveBuildRun(param)
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
	if br.Name != "existing-br" {
		t.Errorf("expected to fetch existing-br, got: %s", br.Name)
	}

	// no BuildRun should have been created
	list, err := shpclientset.ShipwrightV1beta1().BuildRuns(metav1.NamespaceDefault).List(u.cmd.Context(), metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 {
		t.Errorf("expected exactly 1 BuildRun (the pre-existing one), got: %d", len(list.Items))
	}
}
