package accesstoken

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPayload_Scopes(t *testing.T) {
	for _, tc := range []struct {
		scp  string
		want []string
	}{
		{`applied-permissions/user system:metrics:r`, []string{`applied-permissions/user`, `system:metrics:r`}},
		{`foo bar-baz bla123 "hello world"`, []string{`foo`, `bar-baz`, `bla123`, `"hello world"`}},
		{`applied-permissions/groups:group_1,"group 2"`, []string{`applied-permissions/groups:group_1,"group 2"`}},
		{`"applied-permissions/groups:group_1, group 2"`, []string{`"applied-permissions/groups:group_1, group 2"`}},
		{`applied-permissions/groups:"group_1, group 2"`, []string{`applied-permissions/groups:"group_1, group 2"`}},
		{`artifact:"generic-local/foo/bar baz/bla.txt":r,w system:metrics:r`, []string{`artifact:"generic-local/foo/bar baz/bla.txt":r,w`, `system:metrics:r`}},
		{`"artifact:generic-local/foo/bar baz/bla.txt:r,w"`, []string{`"artifact:generic-local/foo/bar baz/bla.txt:r,w"`}},
		{`"artifact:myrepo/foo\"bar.txt:r" applied-permissions/user`, []string{`"artifact:myrepo/foo\"bar.txt:r"`, `applied-permissions/user`}},
		{"  a \t b\n", []string{`a`, `b`}},
		{`a\ b`, []string{`a\ b`}},
		{`a "b c`, []string{`a`, `"b c`}},
		{`a "" b`, []string{`a`, `b`}},
		{`a\`, []string{`a\`}},
		{`   `, nil},
		{``, nil},
	} {
		p := Payload{Scp: tc.scp}
		assert.Equal(t, tc.want, p.Scopes(), tc.scp)
	}
}

func TestUnquoteScope(t *testing.T) {
	for _, tc := range []struct {
		scope string
		want  string
	}{
		{`applied-permissions/user`, `applied-permissions/user`},
		{`"applied-permissions/groups:group_1, group 2"`, `applied-permissions/groups:group_1, group 2`},
		{`applied-permissions/groups:group_1,"group 2"`, `applied-permissions/groups:group_1,group 2`},
		{`artifact:"generic-local/foo/bar baz/bla.txt":r,w`, `artifact:generic-local/foo/bar baz/bla.txt:r,w`},
		{`"artifact:myrepo/foo\"bar.txt:r"`, `artifact:myrepo/foo"bar.txt:r`},
		{`a\\b`, `a\b`},
		{`a\`, `a\`},
		{`"b c`, `b c`},
		{`""`, ``},
	} {
		assert.Equal(t, tc.want, UnquoteScope(tc.scope), tc.scope)
	}
}
