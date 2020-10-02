package roadrunner

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_Pipe_Start(t *testing.T) {
	cmd := exec.Command("php", "tests/client.php", "echo", "pipes")

	ctx := context.Background()
	w, err := NewPipeFactory().SpawnWorker(ctx, cmd)
	assert.NoError(t, err)
	assert.NotNil(t, w)

	//go func() {
	//	assert.NoError(t, w.Wait())
	//}()

	//go func() {
	//	for  {
	//		select {
	//		case event := <-w.Events():
	//			t.Fatal(event)
	//		}
	//	}
	//	//err := w.Wait()
	//	//if err != nil {
	//	//	b.Errorf("error waiting the WorkerProcess: error %v", err)
	//	//}
	//}()
	assert.NoError(t, w.Stop(ctx))
}

func Test_Pipe_StartError(t *testing.T) {
	cmd := exec.Command("php", "tests/client.php", "echo", "pipes")
	err := cmd.Start()
	if err != nil {
		t.Errorf("error running the command: error %v", err)
	}

	ctx := context.Background()
	w, err := NewPipeFactory().SpawnWorker(ctx, cmd)
	assert.Error(t, err)
	assert.Nil(t, w)
}

func Test_Pipe_PipeError(t *testing.T) {
	cmd := exec.Command("php", "tests/client.php", "echo", "pipes")
	_, err := cmd.StdinPipe()
	if err != nil {
		t.Errorf("error creating the STDIN pipe: error %v", err)
	}

	ctx := context.Background()
	w, err := NewPipeFactory().SpawnWorker(ctx, cmd)
	assert.Error(t, err)
	assert.Nil(t, w)
}

func Test_Pipe_PipeError2(t *testing.T) {
	cmd := exec.Command("php", "tests/client.php", "echo", "pipes")
	_, err := cmd.StdinPipe()
	if err != nil {
		t.Errorf("error creating the STDIN pipe: error %v", err)
	}

	ctx := context.Background()
	w, err := NewPipeFactory().SpawnWorker(ctx, cmd)
	assert.Error(t, err)
	assert.Nil(t, w)
}

func Test_Pipe_Failboot(t *testing.T) {
	cmd := exec.Command("php", "tests/failboot.php")
	ctx := context.Background()
	w, err := NewPipeFactory().SpawnWorker(ctx, cmd)

	assert.Nil(t, w)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failboot")
}

func Test_Pipe_Invalid(t *testing.T) {
	cmd := exec.Command("php", "tests/invalid.php")
	ctx := context.Background()
	w, err := NewPipeFactory().SpawnWorker(ctx, cmd)
	assert.Error(t, err)
	assert.Nil(t, w)
}

func Test_Pipe_Echo(t *testing.T) {
	cmd := exec.Command("php", "tests/client.php", "echo", "pipes")
	ctx := context.Background()
	w, err := NewPipeFactory().SpawnWorker(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	//go func() {
	//	assert.NoError(t, w.Wait())
	//}()

	//go func() {
	//	for  {
	//		select {
	//		case event := <-w.Events():
	//			t.Fatal(event)
	//		}
	//	}
	//	//err := w.Wait()
	//	//if err != nil {
	//	//	b.Errorf("error waiting the WorkerProcess: error %v", err)
	//	//}
	//}()
	defer func() {
		err = w.Stop(ctx)
		if err != nil {
			t.Errorf("error stopping the WorkerProcess: error %v", err)
		}
	}()

	sw, err := NewSyncWorker(w)
	if err != nil {
		t.Fatal(err)
	}

	res, err := sw.Exec(ctx, Payload{Body: []byte("hello")})

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.NotNil(t, res.Body)
	assert.Nil(t, res.Context)

	assert.Equal(t, "hello", res.String())
}

func Test_Pipe_Broken(t *testing.T) {
	cmd := exec.Command("php", "tests/client.php", "broken", "pipes")
	ctx := context.Background()
	w, err := NewPipeFactory().SpawnWorker(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	//go func() {
	//	err := w.Wait()
	//
	//	assert.Error(t, err)
	//	assert.Contains(t, err.Error(), "undefined_function()")
	//}()

	//go func() {
	//	for  {
	//		select {
	//		case event := <-w.Events():
	//			t.Fatal(event)
	//		}
	//	}
	//	//err := w.Wait()
	//	//if err != nil {
	//	//	b.Errorf("error waiting the WorkerProcess: error %v", err)
	//	//}
	//}()
	defer func() {
		time.Sleep(time.Second)
		err = w.Stop(ctx)
		// write |1: broken pipe
		assert.Error(t, err)
	}()

	sw, err := NewSyncWorker(w)
	if err != nil {
		t.Fatal(err)
	}

	res, err := sw.Exec(ctx, Payload{Body: []byte("hello")})

	assert.Error(t, err)
	assert.Nil(t, res.Body)
	assert.Nil(t, res.Context)
}

func Benchmark_Pipe_SpawnWorker_Stop(b *testing.B) {
	f := NewPipeFactory()
	ctx := context.Background()
	for n := 0; n < b.N; n++ {
		cmd := exec.Command("php", "tests/client.php", "echo", "pipes")
		w, err := f.SpawnWorker(ctx, cmd)
		if err != nil {
			b.Fatal(err)
		}

		//go func() {
		//	for  {
		//		select {
		//		case event := <-w.Events():
		//			b.Fatal(event)
		//		}
		//	}
		//	//err := w.Wait()
		//	//if err != nil {
		//	//	b.Errorf("error waiting the WorkerProcess: error %v", err)
		//	//}
		//}()
		//go func() {
		//	if w.Wait() != nil {
		//		b.Fail()
		//	}
		//}()

		err = w.Stop(ctx)
		if err != nil {
			b.Errorf("error stopping the WorkerProcess: error %v", err)
		}
	}
}

func Benchmark_Pipe_Worker_ExecEcho(b *testing.B) {
	cmd := exec.Command("php", "tests/client.php", "echo", "pipes")
	ctx := context.Background()
	w, err := NewPipeFactory().SpawnWorker(ctx, cmd)
	if err != nil {
		b.Fatal(err)
	}


	//go func() {
	//	for  {
	//		select {
	//		case event := <-w.Events():
	//			b.Fatal(event)
	//		}
	//	}
	//	//err := w.Wait()
	//	//if err != nil {
	//	//	b.Errorf("error waiting the WorkerProcess: error %v", err)
	//	//}
	//}()
	defer func() {
		err = w.Stop(ctx)
		if err != nil {
			b.Errorf("error stopping the WorkerProcess: error %v", err)
		}
	}()

	sw, err := NewSyncWorker(w)
	if err != nil {
		b.Fatal(err)
	}

	for n := 0; n < b.N; n++ {
		if _, err := sw.Exec(ctx, Payload{Body: []byte("hello")}); err != nil {
			b.Fail()
		}
	}
}
