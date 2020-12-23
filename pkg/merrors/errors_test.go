// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package merrors_test

import (
	stderrors "errors"
	"testing"

	"github.com/bwplotka/mdox/pkg/merrors"
	pkgerrors "github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestNilMultiError(t *testing.T) {
	require.NoError(t, merrors.New().Err())
	require.NoError(t, merrors.New(nil, nil, nil).Err())

	e := merrors.New()
	e.Add()
	require.NoError(t, e.Err())

	e = merrors.New(nil, nil, nil)
	e.Add()
	require.NoError(t, e.Err())

	e = merrors.New()
	e.Add(nil, nil, nil)
	require.NoError(t, e.Err())

	e = merrors.New(nil, nil, nil)
	e.Add(nil, nil, nil)
	require.NoError(t, e.Err())
}

func TestMultiError(t *testing.T) {
	err := stderrors.New("test1")
	require.Error(t, merrors.New(err).Err())
	require.Error(t, merrors.New(nil, err, nil).Err())

	e := merrors.New(err)
	e.Add()
	require.Error(t, e.Err())

	e = merrors.New(nil, nil, nil)
	e.Add(err)
	require.Error(t, e.Err())

	e = merrors.New(err)
	e.Add(nil, nil, nil)
	require.Error(t, e.Err())

	e = merrors.New(nil, nil, nil)
	e.Add(nil, err, nil)
	require.Error(t, e.Err())

	require.Error(t, func() error {
		return e.Err()
	}())

	require.NoError(t, func() error {
		return merrors.New(nil, nil, nil).Err()
	}())
}

func TestMultiError_Error(t *testing.T) {
	err := stderrors.New("test1")

	require.Equal(t, "test1", merrors.New(err).Err().Error())
	require.Equal(t, "test1", merrors.New(err, nil).Err().Error())
	require.Equal(t, "4 errors: test1; test1; test2; test3", merrors.New(err, err, stderrors.New("test2"), nil, stderrors.New("test3")).Err().Error())
}

type customErr struct{ error }

type customErr2 struct{ error }

type customErr3 struct{ error }

func TestMultiError_As(t *testing.T) {
	err := customErr{error: stderrors.New("err1")}

	require.True(t, stderrors.As(err, &err))
	require.True(t, stderrors.As(err, &customErr{}))

	require.False(t, stderrors.As(err, &customErr2{}))
	require.False(t, stderrors.As(err, &customErr3{}))

	// This is just to show limitation of std As.
	require.False(t, stderrors.As(&err, &err))
	require.False(t, stderrors.As(&err, &customErr{}))
	require.False(t, stderrors.As(&err, &customErr2{}))
	require.False(t, stderrors.As(&err, &customErr3{}))

	e := merrors.New(err).Err()
	require.True(t, stderrors.As(e, &customErr{}))
	same := merrors.New(err).Err()
	require.True(t, stderrors.As(e, &same))
	require.False(t, stderrors.As(e, &customErr2{}))
	require.False(t, stderrors.As(e, &customErr3{}))

	e2 := merrors.New(err, customErr3{error: stderrors.New("some")}).Err()
	require.True(t, stderrors.As(e2, &customErr{}))
	require.True(t, stderrors.As(e2, &customErr3{}))
	require.False(t, stderrors.As(e2, &customErr2{}))

	// Wrapped.
	e3 := pkgerrors.Wrap(merrors.New(err, customErr3{}).Err(), "wrap")
	require.True(t, stderrors.As(e3, &customErr{}))
	require.True(t, stderrors.As(e3, &customErr3{}))
	require.False(t, stderrors.As(e3, &customErr2{}))

	// This is just to show limitation of std As.
	e4 := merrors.New(err, &customErr3{}).Err()
	require.False(t, stderrors.As(e4, &customErr2{}))
	require.False(t, stderrors.As(e4, &customErr3{}))
}

func TestMultiError_Is(t *testing.T) {
	err := customErr{error: stderrors.New("err1")}

	require.True(t, stderrors.Is(err, err))
	require.True(t, stderrors.Is(err, customErr{error: err.error}))
	require.False(t, stderrors.Is(err, &err))
	require.False(t, stderrors.Is(err, customErr{}))
	require.False(t, stderrors.Is(err, customErr{error: stderrors.New("err1")}))
	require.False(t, stderrors.Is(err, customErr2{}))
	require.False(t, stderrors.Is(err, customErr3{}))

	require.True(t, stderrors.Is(&err, &err))
	require.False(t, stderrors.Is(&err, &customErr{error: err.error}))
	require.False(t, stderrors.Is(&err, &customErr2{}))
	require.False(t, stderrors.Is(&err, &customErr3{}))

	e := merrors.New(err).Err()
	require.True(t, stderrors.Is(e, err))
	require.True(t, stderrors.Is(err, customErr{error: err.error}))
	require.True(t, stderrors.Is(e, e))
	require.True(t, stderrors.Is(e, merrors.New(err).Err()))
	require.False(t, stderrors.Is(e, &err))
	require.False(t, stderrors.Is(err, customErr{}))
	require.False(t, stderrors.Is(e, customErr2{}))
	require.False(t, stderrors.Is(e, customErr3{}))

	e2 := merrors.New(err, customErr3{}).Err()
	require.True(t, stderrors.Is(e2, err))
	require.True(t, stderrors.Is(e2, customErr3{}))
	require.True(t, stderrors.Is(e2, merrors.New(err, customErr3{}).Err()))
	require.False(t, stderrors.Is(e2, merrors.New(customErr3{}, err).Err()))
	require.False(t, stderrors.Is(e2, customErr{}))
	require.False(t, stderrors.Is(e2, customErr2{}))

	// Wrapped.
	e3 := pkgerrors.Wrap(merrors.New(err, customErr3{}).Err(), "wrap")
	require.True(t, stderrors.Is(e3, err))
	require.True(t, stderrors.Is(e3, customErr3{}))
	require.False(t, stderrors.Is(e3, customErr{}))
	require.False(t, stderrors.Is(e3, customErr2{}))

	exact := &customErr3{}
	e4 := merrors.New(err, exact).Err()
	require.True(t, stderrors.Is(e4, err))
	require.True(t, stderrors.Is(e4, exact))
	require.True(t, stderrors.Is(e4, merrors.New(err, exact).Err()))
	require.False(t, stderrors.Is(e4, customErr{}))
	require.False(t, stderrors.Is(e4, customErr2{}))
	require.False(t, stderrors.Is(e4, &customErr3{}))
}

func TestMultiError_Count(t *testing.T) {
	err := customErr{error: stderrors.New("err1")}
	merr := merrors.New()
	merr.Add(customErr3{})

	m, ok := merrors.AsMulti(merr.Err())
	require.True(t, ok)
	require.Equal(t, 0, m.Count(err))
	require.Equal(t, 1, m.Count(customErr3{}))

	merr.Add(customErr3{})
	merr.Add(customErr3{})

	m, ok = merrors.AsMulti(merr.Err())
	require.True(t, ok)
	require.Equal(t, 0, m.Count(err))
	require.Equal(t, 3, m.Count(customErr3{}))

	// Nest multi errors with wraps.
	merr2 := merrors.New()
	merr2.Add(customErr3{})
	merr2.Add(customErr3{})
	merr2.Add(customErr3{})

	merr3 := merrors.New()
	merr3.Add(customErr3{})
	merr3.Add(customErr3{})

	// Wrap it so Add cannot add inner errors in.
	merr2.Add(pkgerrors.Wrap(merr3.Err(), "wrap"))
	merr.Add(pkgerrors.Wrap(merr2.Err(), "wrap"))

	m, ok = merrors.AsMulti(merr.Err())
	require.True(t, ok)
	require.Equal(t, 0, m.Count(err))
	require.Equal(t, 8, m.Count(customErr3{}))
}

func TestAsMulti(t *testing.T) {
	err := customErr{error: stderrors.New("err1")}
	merr := merrors.New(err, customErr3{}).Err()
	wrapped := pkgerrors.Wrap(merr, "wrap")

	_, ok := merrors.AsMulti(err)
	require.False(t, ok)

	m, ok := merrors.AsMulti(merr)
	require.True(t, ok)
	require.True(t, stderrors.Is(m, merr))

	m, ok = merrors.AsMulti(wrapped)
	require.True(t, ok)
	require.True(t, stderrors.Is(m, merr))
}
