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
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package log

import (
	"bufio"
	"bytes"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestExport(t *testing.T) {
	ts := newTestLogSpy(t)
	conf := &Config{Level: "debug", DisableTimestamp: true}
	logger, _, _ := InitTestLogger(ts, conf)
	ReplaceGlobals(logger, nil)

	Info("Testing")
	Debug("Testing")
	Warn("Testing")
	Error("Testing")
	ts.assertMessagesContains("log_test.go:")

	ts2 := newTestLogSpy(t)
	logger2, _, _ := InitTestLogger(ts2, conf)
	ReplaceGlobals(logger2, nil)

	newLogger := With(zap.String("name", "tester"), zap.Int64("age", 42))
	newLogger.Info("hello")
	newLogger.Debug("world")
	ts2.assertMessagesContains(`name=tester`)
	ts2.assertMessagesContains(`age=42`)
	ts.assertMessagesNotContains(`name=tester`)
}

func TestReplaceGlobals(t *testing.T) {
	ts := newTestLogSpy(t)
	conf := &Config{Level: "debug", DisableTimestamp: true}
	logger, _, _ := InitTestLogger(ts, conf)
	ReplaceGlobals(logger, nil)

	Info(`foo_1`)
	ts.assertLastMessageContains(`foo_1`)

	ts2 := newTestLogSpy(t)
	logger2, _, _ := InitTestLogger(ts2, conf)
	restoreGlobal := ReplaceGlobals(logger2, nil)

	Info(`foo_2`)
	ts.assertMessagesNotContains(`foo_2`)
	ts.assertLastMessageContains(`foo_1`)
	ts2.assertLastMessageContains(`foo_2`)
	ts2.assertMessagesNotContains(`foo_1`)

	restoreGlobal()
	ts.assertMessagesNotContains(`foo_2`)
	ts.assertLastMessageContains(`foo_1`)
	ts2.assertLastMessageContains(`foo_2`)
	ts2.assertMessagesNotContains(`foo_1`)

	Info(`foo_3`)
	ts.assertMessagesNotContains(`foo_2`)
	ts.assertLastMessageContains(`foo_3`)
	ts2.assertLastMessageContains(`foo_2`)
	ts2.assertMessagesNotContains(`foo_1`)
	ts2.assertMessagesNotContains(`foo_3`)
}

func TestZapTextEncoder(t *testing.T) {
	conf := &Config{Level: "debug", File: FileLogConfig{}, DisableTimestamp: true}

	var buffer bytes.Buffer
	writer := bufio.NewWriter(&buffer)
	encoder := NewTextEncoder(conf)
	logger := zap.New(zapcore.NewCore(encoder, zapcore.AddSync(writer), zapcore.InfoLevel)).Sugar()

	logger.Info("this is a message from zap")
	_ = writer.Flush()
	assert.Equal(t, `[INFO] ["this is a message from zap"]`+"\n", buffer.String())
}

func TestRegisteredTextEncoder(t *testing.T) {
	sink := &testingSink{new(bytes.Buffer)}
	_ = zap.RegisterSink("memory", func(*url.URL) (zap.Sink, error) {
		return sink, nil
	})
	lgc := zap.NewProductionConfig()
	lgc.Encoding = ZapEncodingName
	lgc.OutputPaths = []string{"memory://"}

	lg, err := lgc.Build()
	assert.Nil(t, err)

	lg.Info("this is a message from zap")
	assert.Contains(t, sink.String(), `["this is a message from zap"]`)
}

// testingSink implements zap.Sink by writing all messages to a buffer.
type testingSink struct {
	*bytes.Buffer
}

// Implement Close and Sync as no-ops to satisfy the interface. The Write
// method is provided by the embedded buffer.
func (s *testingSink) Close() error { return nil }
func (s *testingSink) Sync() error  { return nil }
