package exec

import (
	"bufio"
	"context"
	"encoding/json"
	"io/fs"
	"os/exec"
	"strconv"
	"time"

	wpfs "github.com/wirepanel/wirepanel/agent/internal/fs"
	"github.com/wirepanel/wirepanel/agent/internal/svc"
	"github.com/wirepanel/wirepanel/agent/internal/sysinfo"
	"github.com/wirepanel/wirepanel/shared/proto"
)

type LogFunc func(stream, data string)

type Result struct {
	ExitCode int
	Data     json.RawMessage
	Err      error
}

type Executor struct{}

func New() *Executor {
	return &Executor{}
}

func (e *Executor) Run(ctx context.Context, task proto.TaskDispatchPayload, logFn LogFunc) Result {
	switch task.Kind {
	case proto.TaskShell:
		return e.runShell(ctx, task, logFn)
	case proto.TaskFSList:
		return wrap(wpfs.List(task.Args["path"]))
	case proto.TaskFSRead:
		return wrap(wpfs.Read(task.Args["path"]))
	case proto.TaskFSWrite:
		return wrap(wpfs.Write(task.Args["path"], task.Body, parseMode(task.Args["mode"])))
	case proto.TaskFSDelete:
		return wrap(wpfs.Delete(task.Args["path"], task.Args["recursive"] == "true", task.Args["trash_id"], task.Args["trash_dir"]))
	case proto.TaskFSMkdir:
		return wrap(wpfs.Mkdir(task.Args["path"], parseMode(task.Args["mode"])))
	case proto.TaskFSStat:
		return wrap(wpfs.Stat(task.Args["path"]))
	case proto.TaskFSRestore:
		return wrap(wpfs.Restore(task.Args["trash_path"], task.Args["original_path"]))
	case proto.TaskServiceList:
		return wrap(svc.List(ctx))
	case proto.TaskServiceCtl:
		return wrap(svc.Action(ctx, task.Args["name"], task.Args["action"]))
	case proto.TaskServiceState:
		return wrap(svc.State(ctx, task.Args["name"]))
	case proto.TaskSystemInfo:
		return wrap(sysinfo.Collect())
	default:
		return Result{ExitCode: 1, Err: &UnsupportedError{Kind: string(task.Kind)}}
	}
}

func wrap(data json.RawMessage, err error) Result {
	if err != nil {
		return Result{ExitCode: 1, Err: err}
	}
	return Result{ExitCode: 0, Data: data}
}

func parseMode(s string) fs.FileMode {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return 0
	}
	return fs.FileMode(v)
}

func (e *Executor) runShell(ctx context.Context, task proto.TaskDispatchPayload, logFn LogFunc) Result {
	timeout := 60 * time.Second
	if task.Timeout > 0 {
		timeout = time.Duration(task.Timeout) * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", task.Command)
	cmd.Dir = task.Args["cwd"]

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Result{ExitCode: 1, Err: err}
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return Result{ExitCode: 1, Err: err}
	}

	if err := cmd.Start(); err != nil {
		return Result{ExitCode: 1, Err: err}
	}

	done := make(chan struct{})
	go func() {
		s := bufio.NewScanner(stdout)
		s.Buffer(make([]byte, 64*1024), 1024*1024)
		for s.Scan() {
			logFn("stdout", s.Text()+"\n")
		}
		done <- struct{}{}
	}()
	go func() {
		s := bufio.NewScanner(stderr)
		s.Buffer(make([]byte, 64*1024), 1024*1024)
		for s.Scan() {
			logFn("stderr", s.Text()+"\n")
		}
		done <- struct{}{}
	}()

	<-done
	<-done

	err = cmd.Wait()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return Result{ExitCode: exitErr.ExitCode()}
		}
		return Result{ExitCode: 1, Err: err}
	}
	return Result{ExitCode: 0}
}

type UnsupportedError struct {
	Kind string
}

func (e *UnsupportedError) Error() string { return "unsupported task kind: " + e.Kind }
