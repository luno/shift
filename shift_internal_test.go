package shift

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type x struct {
	i int
}

type y struct {
	s string
}

type yy y

func Test(t *testing.T) {
	cases := []struct {
		name string
		a    interface{}
		b    interface{}
		res  bool
	}{
		{
			name: "ints",
			a:    int(0),
			b:    int(1),
			res:  true,
		}, {
			name: "struct",
			a:    x{1},
			b:    x{2},
			res:  true,
		}, {
			name: "struct",
			a:    y{"s"},
			b:    y{},
			res:  true,
		}, {
			name: "struct",
			a:    y{"s"},
			b:    yy{},
			res:  false,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.res, sameType(test.a, test.b))
		})
	}
}
