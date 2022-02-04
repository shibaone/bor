package cli

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/internal/cli/server"
	"github.com/ethereum/go-ethereum/log"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
)

var currentDir string = ""
var initialPort uint64 = 60000

// nextPort gives the next available port starting from 60000
func nextPort() uint64 {
	log.Info("Checking for new port", "current", initialPort)
	port := atomic.AddUint64(&initialPort, 1)
	addr := fmt.Sprintf("localhost:%d", port)
	lis, err := net.Listen("tcp", addr)
	if err == nil {
		lis.Close()
		return port
	} else {
		return nextPort()
	}
}

func TestCommand_DebugBlock(t *testing.T) {
	// Start a blockchain in developer mode and get trace of block
	config := server.DefaultConfig()

	// enable developer mode
	config.Developer.Enabled = true
	config.Developer.Period = 2 // block time

	// enable archive mode for getting traces of ancient blocks
	config.GcMode = "archive"

	// grpc port
	port := strconv.Itoa(int(nextPort()))
	log.Info("grpc port", "port", port)
	config.GRPC.Addr = ":" + port

	// datadir
	datadir, _ := ioutil.TempDir("/tmp", "bor-cli-test")
	config.DataDir = datadir
	defer os.RemoveAll(datadir)

	srv, err := server.NewServer(config)
	assert.NoError(t, err)

	// wait for 4 seconds to mine a 2 blocks
	time.Sleep(2 * time.Duration(config.Developer.Period) * time.Second)

	// add prefix for debug trace
	prefix := "bor-block-trace-"

	// output dir
	output := "debug_block_test"

	// set current directory
	currentDir, _ = os.Getwd()

	// trace 1st block
	start := time.Now()
	dst1 := path.Join(output, prefix+time.Now().UTC().Format("2006-01-02-150405Z"), "block.json")
	res := traceBlock(port, 1, output)
	assert.Equal(t, 0, res)
	t.Logf("Completed trace of block %d in %d ms at %s", 1, time.Since(start).Milliseconds(), dst1)

	// adding this to avoid debug directory name conflicts
	time.Sleep(time.Second)

	// trace last/recent block
	start = time.Now()
	latestBlock := srv.GetLatestBlockNumber().Int64()
	dst2 := path.Join(output, prefix+time.Now().UTC().Format("2006-01-02-150405Z"), "block.json")
	res = traceBlock(port, latestBlock, output)
	assert.Equal(t, 0, res)
	t.Logf("Completed trace of block %d in %d ms at %s", latestBlock, time.Since(start).Milliseconds(), dst2)

	// verify if the trace files are created
	done := verify(dst1)
	assert.Equal(t, true, done)
	done = verify(dst2)
	assert.Equal(t, true, done)

	// delete the traces
	deleteTraces(output)
}

// traceBlock calls the cli command to trace a block
func traceBlock(port string, number int64, output string) int {
	ui := cli.NewMockUi()
	log.Info("Port", "port", port)
	command := &DebugBlockCommand{
		Meta2: &Meta2{
			UI:   ui,
			addr: "127.0.0.1:" + port,
		},
	}

	// run trace (by explicity passing the output directory and grpc address)
	return command.Run([]string{strconv.FormatInt(number, 10), "--output", output, "--address", command.Meta2.addr})
}

// verify checks if the trace file is created at the destination
// directory or not
func verify(dst string) bool {
	dst = path.Join(currentDir, dst)
	log.Info("Verifying trace file", "path", dst)
	if file, err := os.Stat(dst); err == nil {
		// check if the file has content
		if file.Size() > 0 {
			return true
		}
	}
	return false
}

// deleteTraces removes the traces created during the test
func deleteTraces(dst string) {
	dst = path.Join(currentDir, dst)
	os.RemoveAll(dst)
}