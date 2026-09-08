package buildversion

import "testing"

func TestFromExplicit(t *testing.T) {
	info, err := FromExplicit("v1.2.3-beta.1+build.7")
	if err != nil {
		t.Fatal(err)
	}
	if info.Version != "1.2.3-beta.1+build.7" || info.WindowsVersion != "1.2.3.0" {
		t.Fatalf("unexpected info: %+v", info)
	}
}

func TestFromExplicitRejectsInvalidVersion(t *testing.T) {
	for _, value := range []string{"release-1.2", "1.2.3-01"} {
		if _, err := FromExplicit(value); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}

func TestFromDescribe(t *testing.T) {
	info, err := FromDescribe("v1.2.0-70-gd960d93", true)
	if err != nil {
		t.Fatal(err)
	}
	if info.Version != "1.2.0-70-gd960d93-dirty" || info.WindowsVersion != "1.2.0.70" {
		t.Fatalf("unexpected info: %+v", info)
	}
}

func TestFromDescribeExactTag(t *testing.T) {
	info, err := FromDescribe("v2.0.1-0-gabcdef0", false)
	if err != nil {
		t.Fatal(err)
	}
	if info.Version != "2.0.1" || info.WindowsVersion != "2.0.1.0" {
		t.Fatalf("unexpected info: %+v", info)
	}
}
