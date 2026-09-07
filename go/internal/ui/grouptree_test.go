package ui

import "testing"

func TestAliasSegments(t *testing.T) {
	cases := []struct {
		alias string
		want  []string
	}{
		{"srv--nas--x", []string{"srv", "nas", "x"}},
		{"srv--nas", []string{"srv", "nas"}},
		{"srv----nas", []string{"srv", "nas"}},
		{"lonealias", []string{"other", "lonealias"}},
		{"srv--", []string{"other", "srv--"}},
		{"--nas", []string{"other", "--nas"}},
	}
	for _, tc := range cases {
		t.Run(tc.alias, func(t *testing.T) {
			got := AliasSegments(tc.alias)
			if len(got) != len(tc.want) {
				t.Fatalf("AliasSegments(%q) = %v, want %v", tc.alias, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("AliasSegments(%q) = %v, want %v", tc.alias, got, tc.want)
				}
			}
		})
	}
}

func TestBuildGroupTree(t *testing.T) {
	tree := BuildGroupTree([]string{"srv--nas--x", "srv--nas--y", "srv--web", "standalone", "lab--a--b--c"})

	srv, ok := tree.Children["srv"]
	if !ok {
		t.Fatal(`expected top-level group "srv"`)
	}
	nas, ok := srv.Children["nas"]
	if !ok {
		t.Fatal(`expected "srv" to have subgroup "nas"`)
	}
	if nas.Leaves["x"] != "srv--nas--x" || nas.Leaves["y"] != "srv--nas--y" {
		t.Fatalf("nas leaves = %v", nas.Leaves)
	}
	if srv.Leaves["web"] != "srv--web" {
		t.Fatalf(`expected "srv" leaf "web" -> "srv--web", got %v`, srv.Leaves)
	}

	other, ok := tree.Children["other"]
	if !ok {
		t.Fatal(`expected "other" group for the alias with no "--"`)
	}
	if other.Leaves["standalone"] != "standalone" {
		t.Fatalf(`expected "other" leaf "standalone" -> "standalone", got %v`, other.Leaves)
	}

	// Arbitrary depth: lab--a--b--c nests 3 levels deep, "c" is the leaf.
	lab := tree.Children["lab"]
	if lab == nil {
		t.Fatal(`expected top-level group "lab"`)
	}
	a := lab.Children["a"]
	if a == nil {
		t.Fatal(`expected "lab" to have subgroup "a"`)
	}
	b := a.Children["b"]
	if b == nil {
		t.Fatal(`expected "a" to have subgroup "b"`)
	}
	if b.Leaves["c"] != "lab--a--b--c" {
		t.Fatalf(`expected "b" leaf "c" -> "lab--a--b--c", got %v`, b.Leaves)
	}
}
