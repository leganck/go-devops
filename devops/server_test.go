package devops

import "testing"

func TestAlignServerGroups_Match(t *testing.T) {
	servers := [][]Server{
		{{ServerAlias: "a1", ServerID: "1"}},
		{{ServerAlias: "b1", ServerID: "2"}, {ServerAlias: "b2", ServerID: "3"}},
	}
	projects := []string{
		"www_ali-z0-smartpos-svc-erp",
		"www_ali-zd1-smartpos-svc-erp",
	}
	got := AlignServerGroups(servers, projects)
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].GroupName != "z0" || got[0].ProjectName != projects[0] {
		t.Fatalf("g0=%+v", got[0])
	}
	if got[1].GroupName != "zd1" || len(got[1].Servers) != 2 {
		t.Fatalf("g1=%+v", got[1])
	}
}

func TestAlignServerGroups_Mismatch(t *testing.T) {
	servers := [][]Server{
		{{ServerAlias: "a1", ServerID: "1"}},
	}
	projects := []string{
		"www_ali-z0-app",
		"www_ali-zd1-app",
	}
	got := AlignServerGroups(servers, projects)
	if len(got) != 1 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].GroupName != "" || got[0].ProjectName != "" {
		t.Fatalf("should not invent group: %+v", got[0])
	}
}

func TestAlignServerGroups_NoProjects(t *testing.T) {
	servers := [][]Server{{{ServerAlias: "a1", ServerID: "1"}}}
	got := AlignServerGroups(servers, nil)
	if got[0].GroupName != "" {
		t.Fatal("expected empty group")
	}
}
