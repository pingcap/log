// Copyright 2019 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package log

import (
	"bufio"
	"bytes"
	"net/url"
	"strings"

	. "github.com/pingcap/check"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var _ = Suite(&testLogSuite{})

type testLogSuite struct{}

func (t *testLogSuite) TestExport(c *C) {
	conf := &Config{Level: "debug", File: FileLogConfig{}, DisableTimestamp: true}
	lg := newZapTestLogger(conf, c)
	ReplaceGlobals(lg.Logger, nil)
	Info("Testing")
	Debug("Testing")
	Warn("Testing")
	Error("Testing")
	lg.AssertContains("log_test.go:")

	lg = newZapTestLogger(conf, c)
	ReplaceGlobals(lg.Logger, nil)
	logger := With(zap.String("name", "tester"), zap.Int64("age", 42))
	logger.Info("hello")
	logger.Debug("world")
	lg.AssertContains(`name=tester`)
	lg.AssertContains(`age=42`)
}

func (t *testLogSuite) TestZapTextEncoder(c *C) {
	conf := &Config{Level: "debug", File: FileLogConfig{}, DisableTimestamp: true}

	var buffer bytes.Buffer
	writer := bufio.NewWriter(&buffer)
	encoder := newZapTextEncoder(conf)
	lg := zap.
		New(zapcore.NewCore(encoder, zapcore.AddSync(writer), zapcore.InfoLevel)).
		Sugar()

	lg.Info("this is a message from zap")
	writer.Flush()
	c.Assert(buffer.String(), Equals, `[INFO] ["this is a message from zap"]
`)
}

// testingSink implements zap.Sink by writing all messages to a buffer.
type testingSink struct {
	*bytes.Buffer
}

// Implement Close and Sync as no-ops to satisfy the interface. The Write
// method is provided by the embedded buffer.
func (s *testingSink) Close() error { return nil }
func (s *testingSink) Sync() error  { return nil }

func (t *testLogSuite) TestRegisteredTextEncoder(c *C) {
	conf := &Config{Level: "debug", File: FileLogConfig{}, DisableTimestamp: true}
	// register the pingcap-log encoder
	_, _, err := InitLoggerWithWriteSyncer(conf, newTestingWriter(c))
	c.Assert(err, IsNil)

	sink := &testingSink{new(bytes.Buffer)}
	zap.RegisterSink("memory", func(*url.URL) (zap.Sink, error) {
		return sink, nil
	})
	lgc := zap.NewProductionConfig()
	lgc.Encoding = ZapEncodingName
	lgc.OutputPaths = []string{"memory://"}

	lg, err := lgc.Build()
	c.Assert(err, IsNil)

	lg.Info("this is a message from zap")
	c.Assert(strings.Contains(sink.String(), `["this is a message from zap"]`), IsTrue)
}
