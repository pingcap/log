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
	"bytes"
	"fmt"
	"io/ioutil"
	"math"
	"net"
	"os"
	"strings"
	"testing"
	"time"
	"unsafe"

	. "github.com/pingcap/check"
	"github.com/pingcap/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func Test(t *testing.T) {
	TestingT(t)
}

var _ = Suite(&testZapLogSuite{})

type testZapLogSuite struct{}

// testingWriter is a WriteSyncer that writes to the the messages.
type testingWriter struct {
	c        *C
	messages []string
}

func newTestingWriter(c *C) *testingWriter {
	return &testingWriter{c: c}
}

func (w *testingWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	p = bytes.TrimRight(p, "\n")
	m := fmt.Sprintf("%s", p)
	w.messages = append(w.messages, m)
	return n, nil
}

func (w *testingWriter) Sync() error {
	return nil
}

type verifyLogger struct {
	*zap.Logger
	w *testingWriter
}

func (v *verifyLogger) AssertMessage(msg ...string) {
	for i, m := range msg {
		v.w.c.Assert(m, Equals, v.w.messages[i])
	}
}

func (v *verifyLogger) AssertContains(substr string) {
	for _, m := range v.w.messages {
		v.w.c.Assert(strings.Contains(m, substr), IsTrue)
	}
}

func (v *verifyLogger) With(fields ...zap.Field) verifyLogger {
	newLg := v.Logger.With(fields...)
	return verifyLogger{
		Logger: newLg,
		w:      v.w,
	}
}

func newZapTestLogger(cfg *Config, c *C, opts ...zap.Option) verifyLogger {
	// TestingWriter is used to write to memory.
	// Used in the verify logger.
	writer := newTestingWriter(c)
	lg, _, err := InitLoggerWithWriteSyncer(cfg, writer, opts...)
	c.Assert(err, IsNil)

	return verifyLogger{
		Logger: lg,
		w:      writer,
	}
}

type username string

func (n username) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("username", string(n))
	return nil
}

func (t *testZapLogSuite) TestLog(c *C) {
	conf := &Config{Level: "debug", File: FileLogConfig{}, DisableTimestamp: true}
	lg := newZapTestLogger(conf, c)
	sugar := lg.Sugar()
	defer sugar.Sync()
	sugar.Infow("failed to fetch URL",
		"url", "http://example.com",
		"attempt", 3,
		"backoff", time.Second,
	)
	sugar.Infof(`failed to "fetch" [URL]: %s`, "http://example.com")
	sugar.Debugw("Slow query",
		"sql", `SELECT * FROM TABLE
	WHERE ID="abc"`,
		"duration", 1300*time.Millisecond,
		"process keys", 1500,
	)
	sugar.Info("Welcome")
	sugar.Info("Welcome TiDB")
	sugar.Info("欢迎")
	sugar.Info("欢迎来到 TiDB")
	sugar.Warnw("Type",
		"Counter", math.NaN(),
		"Score", math.Inf(1),
	)
	lg.With(zap.String("connID", "1"), zap.String("traceID", "dse1121")).Info("new connection")
	lg.Info("Testing typs",
		zap.String("filed1", "noquote"),
		zap.String("filed2", "in quote"),
		zap.Strings("urls", []string{"http://mock1.com:2347", "http://mock2.com:2432"}),
		zap.Strings("urls-peer", []string{"t1", "t2 fine"}),
		zap.Uint64s("store ids", []uint64{1, 4, 5}),
		zap.Object("object", username("user1")),
		zap.Object("object2", username("user 2")),
		zap.Binary("binary", []byte("ab123")),
		zap.Bool("is processed", true),
		zap.ByteString("bytestring", []byte("noquote")),
		zap.ByteString("bytestring", []byte("in quote")),
		zap.Int8("int8", int8(1)),
		zap.Uintptr("ptr", 0xa),
		zap.Reflect("reflect", []int{1, 2}),
		zap.Stringer("stringer", net.ParseIP("127.0.0.1")),
		zap.Bools("array bools", []bool{true}),
		zap.Bools("array bools", []bool{true, true, false}),
		zap.Complex128("complex128", 1+2i),
		zap.Strings("test", []string{
			"💖",
			"�",
			"☺☻☹",
			"日a本b語ç日ð本Ê語þ日¥本¼語i日©",
			"日a本b語ç日ð本Ê語þ日¥本¼語i日©日a本b語ç日ð本Ê語þ日¥本¼語i日©日a本b語ç日ð本Ê語þ日¥本¼語i日©",
			"\x80\x80\x80\x80",
			"<car><mirror>XML</mirror></car>",
		}),
		zap.Duration("duration", 10*time.Second),
	)
	lg.AssertMessage(
		`[INFO] [zap_log_test.go:114] ["failed to fetch URL"] [url=http://example.com] [attempt=3] [backoff=1s]`,
		`[INFO] [zap_log_test.go:119] ["failed to \"fetch\" [URL]: http://example.com"]`,
		`[DEBUG] [zap_log_test.go:120] ["Slow query"] [sql="SELECT * FROM TABLE\n\tWHERE ID=\"abc\""] [duration=1.3s] ["process keys"=1500]`,
		`[INFO] [zap_log_test.go:126] [Welcome]`,
		`[INFO] [zap_log_test.go:127] ["Welcome TiDB"]`,
		`[INFO] [zap_log_test.go:128] [欢迎]`,
		`[INFO] [zap_log_test.go:129] ["欢迎来到 TiDB"]`,
		`[WARN] [zap_log_test.go:130] [Type] [Counter=NaN] [Score=+Inf]`,
		`[INFO] [zap_log_test.go:134] ["new connection"] [connID=1] [traceID=dse1121]`,
		`[INFO] [zap_log_test.go:135] ["Testing typs"] [filed1=noquote] `+
			`[filed2="in quote"] [urls="[http://mock1.com:2347,http://mock2.com:2432]"] `+
			`[urls-peer="[t1,\"t2 fine\"]"] ["store ids"="[1,4,5]"] [object="{username=user1}"] `+
			`[object2="{username=\"user 2\"}"] [binary="YWIxMjM="] ["is processed"=true] `+
			`[bytestring=noquote] [bytestring="in quote"] [int8=1] [ptr=10] [reflect="[1,2]"] [stringer=127.0.0.1] `+
			`["array bools"="[true]"] ["array bools"="[true,true,false]"] [complex128=1+2i] `+
			`[test="[💖,�,☺☻☹,日a本b語ç日ð本Ê語þ日¥本¼語i日©,日a本b語ç日ð本Ê語þ日¥本¼語i日©日a本b語ç日ð本Ê語þ日¥本¼語i日©日a本b語ç日ð本Ê語þ日¥本¼語i日©,\\ufffd\\ufffd\\ufffd\\ufffd,`+
			`<car><mirror>XML</mirror></car>]"] [duration=10s]`,
	)
	c.Assert(func() { sugar.Panic("unknown") }, PanicMatches, `unknown`)
}

func (t *testZapLogSuite) TestTimeEncoder(c *C) {
	sec := int64(1547192741)
	nsec := int64(165279177)
	as, err := time.LoadLocation("Asia/Shanghai")
	c.Assert(err, IsNil)
	tt := time.Unix(sec, nsec).In(as)
	conf := &Config{Level: "debug", File: FileLogConfig{}, DisableTimestamp: true}
	enc := newZapTextEncoder(conf).(*textEncoder)
	DefaultTimeEncoder(tt, enc)
	c.Assert(enc.buf.String(), Equals, `2019/01/11 15:45:41.165 +08:00`)
	enc.buf.Reset()
	utc, err := time.LoadLocation("UTC")
	c.Assert(err, IsNil)
	utcTime := tt.In(utc)
	DefaultTimeEncoder(utcTime, enc)
	c.Assert(enc.buf.String(), Equals, `2019/01/11 07:45:41.165 +00:00`)
}

// See [logger-header]https://github.com/tikv/rfcs/blob/master/text/2018-12-19-unified-log-format.md#log-header-section.
func (t *testZapLogSuite) TestZapCaller(c *C) {
	data := []zapcore.EntryCaller{
		{Defined: true, PC: uintptr(unsafe.Pointer(nil)), File: "server.go", Line: 132},
		{Defined: true, PC: uintptr(unsafe.Pointer(nil)), File: "server/coordinator.go", Line: 20},
		{Defined: true, PC: uintptr(unsafe.Pointer(nil)), File: `z\test_coordinator1.go`, Line: 20},
		{Defined: false, PC: uintptr(unsafe.Pointer(nil)), File: "", Line: 0},
	}
	expect := []string{
		"server.go:132",
		"coordinator.go:20",
		"ztest_coordinator1.go:20",
		"<unknown>",
	}
	conf := &Config{Level: "deug", File: FileLogConfig{}, DisableTimestamp: true}
	enc := newZapTextEncoder(conf).(*textEncoder)

	for i, d := range data {
		ShortCallerEncoder(d, enc)
		c.Assert(enc.buf.String(), Equals, expect[i])
		enc.buf.Reset()
	}
}

func (t *testZapLogSuite) TestRotateLog(c *C) {
	tempDir, _ := ioutil.TempDir("/tmp", "pd-tests-log")
	conf := &Config{
		Level: "info",
		File: FileLogConfig{
			Filename: tempDir + "/test.log",
			MaxSize:  1,
		},
	}
	lg, _, err := InitLogger(conf)
	c.Assert(err, IsNil)
	var data []byte
	for i := 1; i <= 1*1024*1024; i++ {
		if i%1000 != 0 {
			data = append(data, 'd')
			continue
		}
		lg.Info(string(data))
		data = data[:0]
	}
	files, _ := ioutil.ReadDir(tempDir)
	c.Assert(len(files), Equals, 2)
	os.RemoveAll(tempDir)
}

func (t *testZapLogSuite) TestErrorLog(c *C) {
	conf := &Config{Level: "debug", File: FileLogConfig{}, DisableTimestamp: true}
	lg := newZapTestLogger(conf, c)
	lg.Error("", zap.NamedError("err", errors.New("log-stack-test")))
	lg.AssertContains("[err=log-stack-test]")
	lg.AssertContains("] [errVerbose=\"")
}

func (t *testZapLogSuite) TestWithOptions(c *C) {
	conf := &Config{
		Level:               "debug",
		File:                FileLogConfig{},
		DisableTimestamp:    true,
		DisableErrorVerbose: true,
	}
	lg := newZapTestLogger(conf, c, zap.AddStacktrace(zapcore.FatalLevel))
	lg.Error("Testing", zap.Error(errors.New("log-with-option")))
	lg.AssertNotContains("errVerbose")
	lg.AssertNotContains("stack")
}

func (v *verifyLogger) AssertNotContains(substr string) {
	for _, m := range v.w.messages {
		v.w.c.Assert(strings.Contains(m, substr), IsFalse)
	}
}
