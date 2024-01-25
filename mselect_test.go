package mselect

import (
	"fmt"
	"io"
	"sync"
	"testing"
	"time"
)

func TestManySelect_NormalCase(t *testing.T) {
	msel := New()

	type testData struct {
		a [200]byte
		b string
		c int64
	}

	var mu sync.Mutex
	var result struct {
		got1 string
		got2 int64
		got3 testData
		got4 *testData
		got5 any
		got6 fmt.Stringer
		got7 any
	}

	ch1 := make(chan string)
	ch2 := make(chan int64)
	ch3 := make(chan testData)
	ch4 := make(chan *testData)
	ch5 := make(chan any)
	ch6 := make(chan fmt.Stringer)
	ch7 := make(chan *struct{})

	msel.Add(NewTask(ch1,
		func(v string, ok bool) {
			mu.Lock()
			defer mu.Unlock()
			result.got1 = v
		}, nil))
	msel.Add(NewTask(ch2,
		func(v int64, ok bool) {
			assertEqual(t, ok, true)
			mu.Lock()
			defer mu.Unlock()
			result.got2 = v
		}, nil))
	msel.Add(NewTask(ch3, nil, func(v testData, ok bool) {
		assertEqual(t, ok, true)
		mu.Lock()
		defer mu.Unlock()
		result.got3 = v
	}))
	msel.Add(NewTask(ch4, nil, func(v *testData, ok bool) {
		assertEqual(t, ok, true)
		mu.Lock()
		defer mu.Unlock()
		result.got4 = v
	}))
	msel.Add(NewTask(ch5, nil, func(v any, ok bool) {
		assertEqual(t, ok, true)
		mu.Lock()
		defer mu.Unlock()
		result.got5 = v
	}))
	msel.Add(NewTask(ch6, func(v fmt.Stringer, ok bool) {
		assertEqual(t, ok, true)
		mu.Lock()
		defer mu.Unlock()
		result.got6 = v
	}, nil))
	msel.Add(NewTask(ch7, nil, func(v *struct{}, ok bool) {
		assertEqual(t, ok, false)
		mu.Lock()
		defer mu.Unlock()
		result.got7 = v
	}))

	assertEqual(t, msel.Count(), 7)

	time.Sleep(100 * time.Millisecond)

	ch1 <- "ch1 value"
	ch2 <- int64(23456)
	ch3 <- testData{
		a: [200]byte{},
		b: "ch3 value b",
		c: 34567,
	}
	ch4 <- &testData{
		a: [200]byte{},
		b: "ch4 value b",
		c: 45678,
	}
	ch5 <- nil
	ch6 <- stringFunc(func() string { return "stringFunc" })
	close(ch7)

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	copyResult := result
	mu.Unlock()

	assertEqual(t, copyResult.got1, "ch1 value")
	assertEqual(t, copyResult.got2, 23456)
	assertEqual(t, copyResult.got3, testData{
		a: [200]byte{},
		b: "ch3 value b",
		c: 34567,
	})
	assertEqual(t, *copyResult.got4, testData{
		a: [200]byte{},
		b: "ch4 value b",
		c: 45678,
	})
	assertEqual(t, copyResult.got5 == nil, true)
	assertEqual(t, copyResult.got6.String(), "stringFunc")
	assertEqual(t, copyResult.got7 == nil, false)
	assertEqual(t, copyResult.got7.(*struct{}) == nil, true)

	assertEqual(t, msel.Count(), 6)
}

type stringFunc func() string

func (f stringFunc) String() string {
	return f()
}

func TestManySelect_ManyChannels(t *testing.T) {
	N := 5000

	mu := sync.Mutex{}
	result := make([][]int, N)

	makeTask := func(i int) *Task {
		ch := make(chan int)
		task := NewTask(ch, func(v int, ok bool) {
			mu.Lock()
			defer mu.Unlock()
			result[i] = append(result[i], v)
		}, nil)

		go func() {
			time.Sleep(10 * time.Millisecond)
			ch <- i
			time.Sleep(10 * time.Millisecond)
			ch <- i + 1
		}()

		return task
	}

	msel := New()
	for i := 0; i < N; i++ {
		msel.Add(makeTask(i))
	}
	assertEqual(t, msel.Count(), N)

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	copyResult := make([][]int, N)
	copy(copyResult, result)
	mu.Unlock()

	for i := range copyResult {
		assertEqual(t, len(copyResult[i]), 2)
		assertEqual(t, copyResult[i][0], i)
		assertEqual(t, copyResult[i][1], i+1)
	}
}

func TestManySelect_Delete(t *testing.T) {
	msel := New()

	tickers := make([]*time.Ticker, 0)
	tasks := make([]*Task, 0)

	callback := func(t time.Time, ok bool) {
		if !ok {
			return
		}
		io.Discard.Write(t.AppendFormat(nil, time.RFC3339))
	}

	for i := 0; i < 300; i++ {
		ticker := time.NewTicker(80 * time.Millisecond)
		task := NewTask(ticker.C, callback, nil)
		msel.Add(task)
		tickers = append(tickers, ticker)
		tasks = append(tasks, task)
	}
	for i := 300; i < 600; i++ {
		ticker := time.NewTicker(120 * time.Millisecond)
		task := NewTask(ticker.C, nil, callback)
		msel.Add(task)
		tickers = append(tickers, ticker)
		tasks = append(tasks, task)
	}
	assertEqual(t, msel.Count(), 600)

	time.Sleep(500 * time.Millisecond)
	assertEqual(t, msel.Count(), 600)

	for i := 100; i < 120; i++ {
		tickers[i].Stop()
		msel.Delete(tasks[i])
	}
	time.Sleep(50 * time.Millisecond)
	assertEqual(t, msel.Count(), 580)

	for i := 250; i < 280; i++ {
		tickers[i].Stop()
		msel.Delete(tasks[i])
	}
	time.Sleep(50 * time.Millisecond)
	assertEqual(t, msel.Count(), 550)

	for i := 550; i < 600; i++ {
		tickers[i].Stop()
		msel.Delete(tasks[i])
	}
	time.Sleep(50 * time.Millisecond)
	assertEqual(t, msel.Count(), 500)
}

func TestManySelect_Stop(t *testing.T) {
	N := 5000

	mu := sync.Mutex{}
	result := make([][]int, N)

	makeTask := func(i int) *Task {
		ch := make(chan int)
		task := NewTask(ch, func(v int, ok bool) {
			mu.Lock()
			defer mu.Unlock()
			result[i] = append(result[i], v)
		}, nil)

		go func() {
			time.Sleep(10 * time.Millisecond)
			ch <- i
			time.Sleep(10 * time.Millisecond)
			ch <- i + 1
		}()

		return task
	}

	msel := New()
	for i := 0; i < N; i++ {
		msel.Add(makeTask(i))
	}
	assertEqual(t, msel.Count(), N)

	time.Sleep(11 * time.Millisecond)
	msel.Stop()

	time.Sleep(100 * time.Millisecond)
	assertEqual(t, msel.Count(), 0)
}

func assertEqual[T comparable](t *testing.T, left, right T) {
	t.Helper()
	if left != right {
		t.Errorf("values not equal, left= %v, right= %v", left, right)
	}
}
