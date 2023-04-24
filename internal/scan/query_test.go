// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scan

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/vuln/internal/client"
	"golang.org/x/vuln/internal/govulncheck"
	"golang.org/x/vuln/internal/osv"
	"golang.org/x/vuln/internal/test"
)

func TestRunQuery(t *testing.T) {
	e := &osv.Entry{
		ID: "GO-1999-0001",
		Affected: []osv.Affected{{
			Module: osv.Module{Path: "bad.com"},
			Ranges: []osv.Range{{
				Type:   osv.RangeTypeSemver,
				Events: []osv.RangeEvent{{Introduced: "0"}, {Fixed: "1.2.3"}},
			}},
			EcosystemSpecific: osv.EcosystemSpecific{
				Packages: []osv.Package{{
					Path: "bad.com",
				}, {
					Path: "bad.com/bad",
				}},
			},
		}, {
			Module: osv.Module{Path: "unfixable.com"},
			Ranges: []osv.Range{{
				Type:   osv.RangeTypeSemver,
				Events: []osv.RangeEvent{{Introduced: "0"}}, // no fix
			}},
			EcosystemSpecific: osv.EcosystemSpecific{
				Packages: []osv.Package{{
					Path: "unfixable.com",
				}},
			},
		}},
	}
	e2 := &osv.Entry{
		ID: "GO-1999-0002",
		Affected: []osv.Affected{{
			Module: osv.Module{Path: "bad.com"},
			Ranges: []osv.Range{{
				Type:   osv.RangeTypeSemver,
				Events: []osv.RangeEvent{{Introduced: "0"}, {Fixed: "1.2.0"}},
			}},
			EcosystemSpecific: osv.EcosystemSpecific{
				Packages: []osv.Package{{
					Path: "bad.com/pkg",
				},
				},
			},
		}},
	}
	stdlib := &osv.Entry{
		ID: "GO-2000-0003",
		Affected: []osv.Affected{{
			Module: osv.Module{Path: "stdlib"},
			Ranges: []osv.Range{{
				Type:   osv.RangeTypeSemver,
				Events: []osv.RangeEvent{{Introduced: "0"}, {Fixed: "1.19.4"}},
			}},
			EcosystemSpecific: osv.EcosystemSpecific{
				Packages: []osv.Package{{
					Path: "net/http",
				}},
			},
		}},
	}

	c, err := client.NewInMemoryClient([]*osv.Entry{e, e2, stdlib})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	h := test.NewMockHandler()

	for _, tc := range []struct {
		query string
		want  []*govulncheck.Vuln
	}{
		{
			query: "stdlib@go1.18",
			want: []*govulncheck.Vuln{
				{OSV: stdlib},
			},
		},
		{
			query: "stdlib@1.18",
			want: []*govulncheck.Vuln{
				{OSV: stdlib},
			},
		},
		{
			query: "stdlib@v1.18.0",
			want: []*govulncheck.Vuln{
				{OSV: stdlib},
			},
		},
		{
			query: "bad.com@1.2.3",
			want:  nil,
		},
		{
			query: "bad.com@v1.1.0",
			want: []*govulncheck.Vuln{
				{OSV: e}, {OSV: e2},
			},
		},
		{
			query: "unfixable.com@2.0.0",
			want: []*govulncheck.Vuln{
				{OSV: e},
			},
		},
	} {
		t.Run(tc.query, func(t *testing.T) {
			got, err := runQuery(ctx, h, &config{patterns: []string{tc.query}}, c)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(got, tc.want); diff != "" {
				t.Errorf("runQuery: unexpected diff:\n%s", diff)
			}
		})
	}
}

func TestParseModuleQuery(t *testing.T) {
	for _, tc := range []struct {
		pattern, wantMod, wantVer string
		wantErr                   string
	}{
		{
			pattern: "stdlib@go1.18",
			wantMod: "stdlib",
			wantVer: "go1.18",
		},
		{
			pattern: "golang.org/x/tools@v0.0.0-20140414041502-123456789012",
			wantMod: "golang.org/x/tools",
			wantVer: "v0.0.0-20140414041502-123456789012",
		},
		{
			pattern: "golang.org/x/tools",
			wantErr: "invalid query",
		},
		{
			pattern: "golang.org/x/tools@1.0.0.2",
			wantErr: "not valid semver",
		},
	} {
		t.Run(tc.pattern, func(t *testing.T) {
			gotMod, gotVer, err := parseModuleQuery(tc.pattern)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				if gotMod != tc.wantMod || gotVer != tc.wantVer {
					t.Errorf("parseModuleQuery = (%s, %s), want (%s, %s)", gotMod, gotVer, tc.wantMod, tc.wantVer)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("parseModuleQuery = %v, want err containing %s", err, tc.wantErr)
				}
			}
		})
	}
}
