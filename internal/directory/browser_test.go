package directory

import (
	"context"
	"testing"
)

func TestMockDirectoryBrowser(t *testing.T) {
	svc := NewMockService()
	ctx := context.Background()

	children, err := svc.ListChildren(ctx, "DC=example,DC=com", 50)
	if err != nil || len(children) == 0 {
		t.Fatalf("list children: err=%v count=%d", err, len(children))
	}

	user, err := svc.CreateUser(ctx, CreateUserInput{
		ParentDN: "OU=Employees,DC=example,DC=com",
		Name:     "Taylor Swift",
		Attributes: map[string]string{
			"mail": "taylor@example.com", "displayName": "Taylor Swift",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	group, err := svc.CreateGroup(ctx, CreateGroupInput{
		ParentDN: "OU=Groups,DC=example,DC=com", Name: "Artists", Description: "Artist accounts",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AddMember(ctx, group.DistinguishedName, user.DistinguishedName); err != nil {
		t.Fatal(err)
	}
	members, err := svc.ListMembers(ctx, group.DistinguishedName)
	if err != nil || len(members) != 1 {
		t.Fatalf("members err=%v count=%d", err, len(members))
	}
	updated, err := svc.UpdateAttributes(ctx, user.DistinguishedName, map[string]string{"title": "Singer"})
	if err != nil || updated.Attributes["title"] != "Singer" {
		t.Fatalf("update attrs: %#v err=%v", updated.Attributes, err)
	}
	if err := svc.SetUserEnabled(ctx, user.DistinguishedName, false); err != nil {
		t.Fatal(err)
	}
	entry, err := svc.GetEntry(ctx, user.DistinguishedName)
	if err != nil || entry.Enabled == nil || *entry.Enabled {
		t.Fatalf("expected disabled user, entry=%#v err=%v", entry, err)
	}
	if err := svc.RemoveMember(ctx, group.DistinguishedName, user.DistinguishedName); err != nil {
		t.Fatal(err)
	}
}

func TestFilterAttributesRejectsSecrets(t *testing.T) {
	if _, err := FilterAttributes(map[string]string{"userPassword": "x"}, nil); err == nil {
		t.Fatal("expected secret attribute rejection")
	}
	if _, err := FilterAttributes(map[string]string{"notAllowed": "x"}, nil); err == nil {
		t.Fatal("expected allowlist rejection")
	}
}

func TestClassifyEntryOpenLDAPPersonIsUser(t *testing.T) {
	kind := classifyEntry([]string{"top", "person", "organizationalPerson", "inetOrgPerson"})
	if kind != NodeUser {
		t.Fatalf("expected user, got %s", kind)
	}
	if classifyEntry([]string{"top", "organizationalUnit"}) != NodeOU {
		t.Fatal("expected ou")
	}
	if classifyEntry([]string{"top", "organization"}) != NodeContainer {
		t.Fatal("expected organization container")
	}
}
