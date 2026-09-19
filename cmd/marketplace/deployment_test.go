package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// surfaces are the two places this one binary runs: the service, and the job
// that applies migrations and declares the marketplaces.
var surfaces = map[string]string{
	"the service":       filepath.Join("..", "..", "infra", "terraform", "cloud_run.tf"),
	"the migration job": filepath.Join("..", "..", "infra", "terraform", "migrate_job.tf"),
}

// required is every variable the binary refuses to start without when the
// deployment talks to real providers (internal/platform/config).
//
// The list is written out rather than derived, because what it is checking is
// that two people agreed: the code that demands a variable, and the
// infrastructure that supplies it.
var required = []string{
	"PROVIDERS_MODE",
	"MAIL_API_KEY",
	"MAIL_FROM",
	"AUDIT_KEY",
}

// TestBothDeploymentSurfacesGetWhatTheBinaryDemands is a lesson rather than a
// precaution.
//
// `AUDIT_KEY` was declared on the service and not on the job. The job runs the
// same binary, in the same mode, and that binary refuses to start without the
// variable — by design. So the deployment failed at the migration step, with
// everything green in the pull request that caused it: nothing in a test or a
// plan says that a variable one surface needs is missing from the other.
func TestBothDeploymentSurfacesGetWhatTheBinaryDemands(t *testing.T) {
	for surface, path := range surfaces {
		declared, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("cannot read the Terraform of %s: %v", surface, err)
		}

		for _, variable := range required {
			if !strings.Contains(string(declared), `"`+variable+`"`) {
				t.Errorf("%s does not declare %s, and the binary refuses to start without it",
					surface, variable)
			}
		}
	}
}
