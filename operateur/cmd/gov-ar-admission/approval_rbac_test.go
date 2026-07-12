package main

import (
	"os"
	"strings"
	"testing"
)

func TestAdmissionServiceAccountCannotForgeHumanDecisionOrApprovalStatus(t *testing.T) {
	raw, err := os.ReadFile("../../charts/ai-sovereign-finops-operator/templates/rbac.yaml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	roleStart := strings.Index(text, "name: {{ include \"operator.fullname\" . }}-gov-ar-admission\n")
	if roleStart < 0 {
		t.Fatal("admission ClusterRole missing")
	}
	roleEndOffset := strings.Index(text[roleStart:], "kind: ClusterRoleBinding")
	if roleEndOffset < 0 {
		t.Fatal("admission ClusterRoleBinding boundary missing")
	}
	role := text[roleStart : roleStart+roleEndOffset]
	if !strings.Contains(role, "resources: [\"aiadmissionapprovals\"]\n  verbs: [\"create\", \"get\", \"list\"]") {
		t.Fatal("admission role lacks proposal-only create/read permission")
	}
	for _, forbidden := range []string{"aiadmissionapprovaldecisions", "aichangerequests", "aiadmissionapprovals/status", `"update"`, `"patch"`, `"delete"`} {
		if strings.Contains(role, forbidden) {
			t.Fatalf("admission role contains forbidden authority %q", forbidden)
		}
	}
	reviewerRole := "name: {{ include \"operator.fullname\" . }}-gov-ar-approval-reviewer"
	if strings.Count(text, reviewerRole) != 1 {
		t.Fatalf("reviewer role occurrences=%d; expected one unbound role", strings.Count(text, reviewerRole))
	}
}
