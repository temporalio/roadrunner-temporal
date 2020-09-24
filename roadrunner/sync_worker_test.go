package roadrunner

import (
	"github.com/stretchr/testify/assert"
	"os/exec"
	"testing"
	"time"
)

func Test_Echo(t *testing.T) {
	cmd := exec.Command("php", "tests/client.php", "echo", "pipes")

	w, _ := NewPipeFactory().SpawnWorker(cmd)
	go func() {
		assert.NoError(t, w.Wait())
	}()
	defer func() {
		err := w.Stop()
		if err != nil {
			t.Errorf("error stopping the WorkerProcess: error %v", err)
		}
	}()

	res, err := w.Exec(&Payload{Body: []byte("hello")})

	assert.Nil(t, err)
	assert.NotNil(t, res)
	assert.NotNil(t, res.Body)
	assert.Nil(t, res.Context)

	assert.Equal(t, "hello", res.String())
}

func Test_BadPayload(t *testing.T) {
	cmd := exec.Command("php", "tests/client.php", "echo", "pipes")

	w, _ := NewPipeFactory().SpawnWorker(cmd)
	go func() {
		assert.NoError(t, w.Wait())
	}()
	defer func() {
		err := w.Stop()
		if err != nil {
			t.Errorf("error stopping the WorkerProcess: error %v", err)
		}
	}()

	res, err := w.Exec(nil)

	assert.Error(t, err)
	assert.Nil(t, res)

	assert.Equal(t, "payload can not be empty", err.Error())
}

func Test_NotStarted_String(t *testing.T) {
	cmd := exec.Command("php", "tests/client.php", "echo", "pipes")

	w, _ := initWorker(cmd)
	assert.Contains(t, w.String(), "php tests/client.php echo pipes")
	assert.Contains(t, w.String(), "inactive")
	assert.Contains(t, w.String(), "numExecs: 0")
}

func Test_NotStarted_Exec(t *testing.T) {
	cmd := exec.Command("php", "tests/client.php", "echo", "pipes")

	w, _ := initWorker(cmd)

	res, err := w.Exec(&Payload{Body: []byte("hello")})

	assert.Error(t, err)
	assert.Nil(t, res)

	assert.Equal(t, "WorkerProcess is not ready (inactive)", err.Error())
}

func Test_String(t *testing.T) {
	cmd := exec.Command("php", "tests/client.php", "echo", "pipes")

	w, _ := NewPipeFactory().SpawnWorker(cmd)
	go func() {
		assert.NoError(t, w.Wait())
	}()
	defer func() {
		err := w.Stop()
		if err != nil {
			t.Errorf("error stopping the WorkerProcess: error %v", err)
		}
	}()

	assert.Contains(t, w.String(), "php tests/client.php echo pipes")
	assert.Contains(t, w.String(), "ready")
	assert.Contains(t, w.String(), "numExecs: 0")
}

func Test_Echo_Slow(t *testing.T) {
	cmd := exec.Command("php", "tests/slow-client.php", "echo", "pipes", "10", "10")

	w, _ := NewPipeFactory().SpawnWorker(cmd)
	go func() {
		assert.NoError(t, w.Wait())
	}()
	defer func() {
		err := w.Stop()
		if err != nil {
			t.Errorf("error stopping the WorkerProcess: error %v", err)
		}
	}()

	res, err := w.Exec(&Payload{Body: []byte("hello")})

	assert.Nil(t, err)
	assert.NotNil(t, res)
	assert.NotNil(t, res.Body)
	assert.Nil(t, res.Context)

	assert.Equal(t, "hello", res.String())
}

func Test_Broken(t *testing.T) {
	cmd := exec.Command("php", "tests/client.php", "broken", "pipes")

	w, err := NewPipeFactory().SpawnWorker(cmd)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		err := w.Wait()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "undefined_function()")
	}()

	res, err := w.Exec(&Payload{Body: []byte("hello")})
	assert.Nil(t, res)
	assert.NotNil(t, err)

	time.Sleep(time.Second)
	assert.NoError(t, w.Stop())
}

func Test_Error(t *testing.T) {
	cmd := exec.Command("php", "tests/client.php", "error", "pipes")

	w, _ := NewPipeFactory().SpawnWorker(cmd)
	go func() {
		assert.NoError(t, w.Wait())
	}()

	defer func() {
		err := w.Stop()
		if err != nil {
			t.Errorf("error stopping the WorkerProcess: error %v", err)
		}
	}()

	res, err := w.Exec(&Payload{Body: []byte("hello")})
	assert.Nil(t, res)
	assert.NotNil(t, err)

	assert.IsType(t, TaskError{}, err)
	assert.Equal(t, "hello", err.Error())
}

func Test_NumExecs(t *testing.T) {
	cmd := exec.Command("php", "tests/client.php", "echo", "pipes")

	w, _ := NewPipeFactory().SpawnWorker(cmd)
	go func() {
		assert.NoError(t, w.Wait())
	}()
	defer func() {
		err := w.Stop()
		if err != nil {
			t.Errorf("error stopping the WorkerProcess: error %v", err)
		}
	}()

	_, err := w.Exec(&Payload{Body: []byte("hello")})
	if err != nil {
		t.Errorf("fail to execute payload: error %v", err)
	}
	assert.Equal(t, int64(1), w.State().NumExecs())

	_, err = w.Exec(&Payload{Body: []byte("hello")})
	if err != nil {
		t.Errorf("fail to execute payload: error %v", err)
	}
	assert.Equal(t, int64(2), w.State().NumExecs())

	_, err = w.Exec(&Payload{Body: []byte("hello")})
	if err != nil {
		t.Errorf("fail to execute payload: error %v", err)
	}
	assert.Equal(t, int64(3), w.State().NumExecs())
}
