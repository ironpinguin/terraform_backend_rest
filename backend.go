package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
)

type ConflictError struct {
	StatusCode int
}

func (c *ConflictError) Error() string {
	return fmt.Sprintf("status %d", c.StatusCode)
}

type FileNotExistsError struct {
	Info string
}

func (n *FileNotExistsError) Error() string {
	return n.Info
}

type LockInfo struct {
	// Unique ID for the lock. NewLockInfo provides a random ID, but this may
	// be overridden by the lock implementation. The final value of ID will be
	// returned by the call to Lock.
	ID string

	// Terraform operation, provided by the caller.
	Operation string

	// Extra information to store with the lock, provided by the caller.
	Info string

	// user@hostname when available
	Who string

	// Terraform version
	Version string

	// Time that the lock was taken.
	Created time.Time

	// Path to the state file when applicable. Set by the Lock implementation.
	Path string
}

type Backend struct {
	dir string
}

func (b *Backend) get(tf_id string) ([]byte, error) {
	var tfstate_filename = b.dir + tf_id + ".tfstate"
	var tfstate []byte
	var err error

	if _, err := os.Stat(tfstate_filename); os.IsNotExist(err) {
		log.Infof("File %s not found", tfstate_filename)
		return nil, err
	}
	if tfstate, err = ioutil.ReadFile(tfstate_filename); err != nil {
		log.Warnf("Can't read file %s. With follow error %v", tfstate_filename, err)
		return nil, err
	}

	return tfstate, nil
}

func (b *Backend) update(tf_id string, tfstate []byte) error {
	var tfstate_filename = b.dir + tf_id + ".tfstate"

	if err := ioutil.WriteFile(tfstate_filename, tfstate, 0644); err != nil {
		log.Warnf("Can't write file %s. Got follow error %v", tfstate_filename, err)
		return err
	}
	return nil
}

func (b *Backend) pruge(tf_id string) error {
	var tfstate_filename = b.dir + tf_id + ".tfstate"

	if _, err := os.Stat(tfstate_filename); os.IsNotExist(err) {
		log.Infof("File %s not found", tfstate_filename)
		return nil
	}

	if err := os.Remove(tfstate_filename); err != nil {
		log.Warnf("Can't delete file %s. Got follow error %v", tfstate_filename, err)
		return err
	}
	return nil
}

func (b *Backend) lock(tf_id string, lock []byte) ([]byte, error) {
	var lock_filename string = b.dir + tf_id + ".lock"
	var lock_file []byte
	var lock_info, current_lock_info LockInfo
	var err error

	if err := json.Unmarshal(lock, &lock_info); err != nil {
		log.Errorf("unexpected decoding json error %v", err)
		return nil, err
	}
	if _, err := os.Stat(lock_filename); os.IsNotExist(err) {
		if lock_file, err = json.MarshalIndent(lock_info, "", " "); err != nil {
			log.Errorf("unexpected encoding json error %v", err)
			return nil, err
		}
		if err = ioutil.WriteFile(lock_filename, lock_file, 0644); err != nil {
			log.Errorf("Can't write lock file %s. Got follow error %v", lock_filename, err)
			return nil, err
		}
		return lock_file, nil
	}

	if lock_file, err = ioutil.ReadFile(lock_filename); err != nil {
		log.Errorf("Can't read file %s. With follow error %v", lock_filename, err)
		return nil, err
	}
	if err := json.Unmarshal(lock_file, &current_lock_info); err != nil {
		log.Errorf("unexpected decoding json error %v", err)
		return nil, err
	}
	if current_lock_info.ID != lock_info.ID {
		log.Infof("state is locked with diffrend id %s, but follow id requestd lock %s", current_lock_info.ID, lock_info.ID)
		return nil, &ConflictError{
			StatusCode: http.StatusConflict,
		}
	}
	return lock_file, nil
}

func (b *Backend) unlock(tf_id string, lock []byte) error {
	var lock_filename string = b.dir + tf_id + ".lock"
	var lock_file []byte
	var err error
	var lock_info, current_lock_info LockInfo

	if err := json.Unmarshal(lock, &lock_info); err != nil {
		log.Errorf("unexpected decoding json error %v", err)
		return err
	}
	if _, err := os.Stat(lock_filename); os.IsNotExist(err) {
		log.Infof("lock file %s is deleted so notting to do.", lock_filename)
		return nil
	}
	if lock_file, err = ioutil.ReadFile(lock_filename); err != nil {
		log.Errorf("Can't read file %s. With follow error %v", lock_filename, err)
		return err
	}
	if err := json.Unmarshal(lock_file, &current_lock_info); err != nil {
		log.Errorf("unexpected decoding json error %v", err)
		return err
	}
	if current_lock_info.ID != lock_info.ID {
		log.Infof("state is locked with diffrend id %s, but follow id requestd lock %s", current_lock_info.ID, lock_info.ID)
		return &ConflictError{
			StatusCode: http.StatusConflict,
		}
	}
	if err := os.Remove(lock_filename); err != nil {
		log.Warnf("Can't delete file %s. Got follow error %v", lock_filename, err)
		return err
	}

	return nil
}
