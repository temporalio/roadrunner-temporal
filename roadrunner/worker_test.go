package roadrunner

import (
	"github.com/stretchr/testify/assert"
	"os/exec"
	"testing"
	"time"
)

func Test_GetState(t *testing.T) {
	cmd := exec.Command("php", "tests/client.php", "echo", "pipes")

	w, err := NewPipeFactory().SpawnWorker(cmd)
	go func() {
		assert.NoError(t, w.Wait())
		assert.Equal(t, StateStopped, w.State().Value())
	}()

	assert.NoError(t, err)
	assert.NotNil(t, w)

	assert.Equal(t, StateReady, w.State().Value())
	err = w.Stop()
	if err != nil {
		t.Errorf("error stopping the WorkerProcess: error %v", err)
	}
}

func Test_Kill(t *testing.T) {
	cmd := exec.Command("php", "tests/client.php", "echo", "pipes")

	w, err := NewPipeFactory().SpawnWorker(cmd)
	go func() {
		assert.Error(t, w.Wait())
		assert.Equal(t, StateStopped, w.State().Value())
	}()

	assert.NoError(t, err)
	assert.NotNil(t, w)

	assert.Equal(t, StateReady, w.State().Value())
	defer func() {
		err := w.Kill()
		if err != nil {
			t.Errorf("error killing the WorkerProcess: error %v", err)
		}
	}()
}

func Test_OnStarted(t *testing.T) {
	cmd := exec.Command("php", "tests/client.php", "broken", "pipes")
	assert.Nil(t, cmd.Start())

	w, err := initWorker(cmd)
	assert.Nil(t, w)
	assert.NotNil(t, err)

	assert.Equal(t, "can't attach to running process", err.Error())
}

func TestErrBuffer_Write_Len(t *testing.T) {
	buf := newErrBuffer(nil)
	defer func() {
		err := buf.Close()
		if err != nil {
			t.Errorf("error during closing the buffer: error %v", err)
		}
	}()

	_, err := buf.Write([]byte("hello"))
	if err != nil {
		t.Errorf("fail to write: error %v", err)
	}
	assert.Equal(t, 5, buf.Len())
	assert.Equal(t, "hello", buf.String())
}

func TestErrBuffer_Write_Event(t *testing.T) {
	buf := newErrBuffer(nil)
	defer func() {
		err := buf.Close()
		if err != nil {
			t.Errorf("error during closing the buffer: error %v", err)
		}
	}()

	tr := make(chan interface{})
	buf.listener = func(event int, ctx interface{}) {
		assert.Equal(t, EventStderrOutput, event)
		assert.Equal(t, []byte("hello\n"), ctx)
		close(tr)
	}

	_, err := buf.Write([]byte("hello\n"))
	if err != nil {
		t.Errorf("fail to write: error %v", err)
	}
	<-tr

	// messages are read
	assert.Equal(t, 0, buf.Len())
}

func TestErrBuffer_Write_Event_Separated(t *testing.T) {
	buf := newErrBuffer(nil)
	defer func() {
		err := buf.Close()
		if err != nil {
			t.Errorf("error during closing the buffer: error %v", err)
		}
	}()

	tr := make(chan interface{})
	buf.listener = func(event int, ctx interface{}) {
		assert.Equal(t, EventStderrOutput, event)
		assert.Equal(t, []byte("hello\nending"), ctx)
		close(tr)
	}

	_, err := buf.Write([]byte("hel"))
	if err != nil {
		t.Errorf("fail to write: error %v", err)
	}

	_, err = buf.Write([]byte("lo\n"))
	if err != nil {
		t.Errorf("fail to write: error %v", err)
	}

	_, err = buf.Write([]byte("ending"))
	if err != nil {
		t.Errorf("fail to write: error %v", err)
	}

	<-tr
	assert.Equal(t, 0, buf.Len())
	assert.Equal(t, "", buf.String())
}

func TestErrBuffer_Write_Event_Separated_NoListener(t *testing.T) {
	buf := newErrBuffer(nil)
	defer func() {
		err := buf.Close()
		if err != nil {
			t.Errorf("error during closing the buffer: error %v", err)
		}
	}()

	_, err := buf.Write([]byte("hel"))
	if err != nil {
		t.Errorf("fail to write: error %v", err)
	}

	_, err = buf.Write([]byte("lo\n"))
	if err != nil {
		t.Errorf("fail to write: error %v", err)
	}

	_, err = buf.Write([]byte("ending"))
	if err != nil {
		t.Errorf("fail to write: error %v", err)
	}

	assert.Equal(t, 12, buf.Len())
	assert.Equal(t, "hello\nending", buf.String())
}

func TestErrBuffer_Write_Remaining(t *testing.T) {
	buf := newErrBuffer(nil)
	defer func() {
		err := buf.Close()
		if err != nil {
			t.Errorf("error during closing the buffer: error %v", err)
		}
	}()

	_, err := buf.Write([]byte("hel"))
	if err != nil {
		t.Errorf("fail to write: error %v", err)
	}

	assert.Equal(t, 3, buf.Len())
	assert.Equal(t, "hel", buf.String())
}
