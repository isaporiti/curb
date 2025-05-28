package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/isaporiti/curb"
	"github.com/isaporiti/vial"
)

func TestWithMaxAttempts(t *testing.T) {
	testCases := []struct {
		desc        string
		maxAttempts int
		want        error
	}{
		{
			desc:        "returns error if max attempts < 0",
			maxAttempts: -1,
			want:        curb.ErrInvalidMaxAttempts,
		},
		{
			desc:        "returns error if max attempts == 0",
			maxAttempts: 0,
			want:        curb.ErrInvalidMaxAttempts,
		},
		{
			desc:        "returns error if max attempts > 10",
			maxAttempts: 11,
			want:        curb.ErrInvalidMaxAttempts,
		},
		{
			desc:        "success",
			maxAttempts: 6,
			want:        nil,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			_, err := curb.Retry(
				context.Background(),
				func() (struct{}, error) { return struct{}{}, nil },
				curb.WithMaxAttempts(tC.maxAttempts),
			)

			vial.True(t, errors.Is(err, tC.want))
		})
	}
}

func TestRetry(t *testing.T) {
	t.Run("it times out by using context", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(
			context.Background(),
			10*time.Millisecond,
		)
		defer cancel()
		fn := func() (struct{}, error) {
			return struct{}{}, errors.New("an error")
		}

		_, err := curb.Retry(ctx, fn)

		vial.True(t, errors.Is(err, context.DeadlineExceeded))
	})

	t.Run("retries 5 times by default", func(t *testing.T) {
		t.Parallel()
		var sleeper fakeSleeper
		attempts := 0
		want := 5
		fn := func() (struct{}, error) {
			attempts++

			if attempts == want {
				return struct{}{}, nil
			}
			return struct{}{}, errors.New("an error")
		}

		_, err := curb.Retry(
			context.Background(),
			fn,
			curb.WithSleeper(sleeper.sleep),
		)

		vial.Equal(t, want, attempts)
		vial.Equal(t, want-1, len(sleeper.delays))
		vial.NoError(t, err)
	})

	t.Run("retries a configurable number of times", func(t *testing.T) {
		t.Parallel()
		var sleeper fakeSleeper
		attempts := 0
		want := 2
		fn := func() (struct{}, error) {
			attempts++

			if attempts == want {
				return struct{}{}, nil
			}
			return struct{}{}, errors.New("an error")
		}
		_, err := curb.Retry(
			context.Background(),
			fn,
			curb.WithMaxAttempts(want),
			curb.WithSleeper(sleeper.sleep),
		)

		vial.Equal(t, want, attempts)
		vial.Equal(t, want-1, len(sleeper.delays))
		vial.NoError(t, err)
	})

	t.Run("returns ErrRetriesExhausted after exhausting retries", func(t *testing.T) {
		t.Parallel()
		var sleeper fakeSleeper

		_, err := curb.Retry(
			context.Background(),
			func() (bool, error) {
				return false, errors.New("persistent failure")
			},
			curb.WithMaxAttempts(1),
			curb.WithSleeper(sleeper.sleep),
		)

		vial.True(t, errors.As(err, &curb.ErrRetriesExhausted{}))
	})

	t.Run("stops retries on context cancel", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		var sleeper fakeSleeper
		attempts := 0
		fn := func() (bool, error) {
			attempts++
			if attempts == 2 {
				cancel()
			}
			return false, errors.New("fail")
		}

		_, err := curb.Retry(ctx, fn, curb.WithSleeper(sleeper.sleep))
		vial.True(t, errors.Is(err, context.Canceled))
	})

	t.Run("WithMaxAttempts(1) only calls fn once", func(t *testing.T) {
		t.Parallel()
		var sleeper fakeSleeper
		calls := 0

		_, _ = curb.Retry(
			context.Background(),
			func() (bool, error) {
				calls++
				return false, errors.New("fail")
			},
			curb.WithMaxAttempts(1),
			curb.WithSleeper(sleeper.sleep),
		)

		vial.Equal(t, 1, calls)
		vial.Equal(t, 0, len(sleeper.delays))
	})

	t.Run("success without retries", func(t *testing.T) {
		t.Parallel()
		var sleeper fakeSleeper

		got, err := curb.Retry(
			context.Background(),
			func() (bool, error) {
				return true, nil
			},
			curb.WithSleeper(sleeper.sleep),
		)

		vial.True(t, got)
		vial.Equal(t, 0, len(sleeper.delays))
		vial.NoError(t, err)
	})
}

type fakeSleeper struct {
	delays []time.Duration
}

func (s *fakeSleeper) sleep(d time.Duration) <-chan time.Time {
	s.delays = append(s.delays, d)
	ch := make(chan time.Time, 1)
	ch <- time.Now()
	return ch
}
