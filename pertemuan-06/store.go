package main

// store.go -- SUDAH LENGKAP, tidak perlu diubah. Ini Task, TaskStore, dan
// MemoryStore yang kalian kenal dari Pertemuan 4, ditambah method Update
// (yang dibutuhkan PUT /tasks/{id}). Yang kalian kerjakan di tugas ini ada di
// api.go: lapisan HTTP yang bicara ke TaskStore (interface), bukan ke
// MemoryStore langsung.

import (
	"errors"
	"strings"
	"sync"
)

var (
	ErrTugasTidakDitemukan = errors.New("tugas tidak ditemukan")
	ErrInputKosong         = errors.New("input tidak boleh kosong")
)

// Task adalah satu tugas. Tag json menentukan nama field di JSON.
type Task struct {
	ID      int    `json:"id"`
	Judul   string `json:"judul"`
	Selesai bool   `json:"selesai"`
}

// TaskStore adalah kontrak penyimpanan. Handler HTTP hanya boleh bergantung
// pada interface ini.
type TaskStore interface {
	Add(t Task) (Task, error)
	Get(id int) (Task, error)
	List() []Task
	Update(t Task) (Task, error)
	Delete(id int) error
}

// MemoryStore menyimpan tugas di memori (aman dipakai banyak goroutine).
type MemoryStore struct {
	mu     sync.Mutex
	tugas  []Task
	nextID int
}

var _ TaskStore = (*MemoryStore)(nil)

func NewMemoryStore() *MemoryStore { return &MemoryStore{nextID: 1} }

func (m *MemoryStore) Add(t Task) (Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t.Judul = strings.TrimSpace(t.Judul)
	if t.Judul == "" {
		return Task{}, ErrInputKosong
	}
	t.ID = m.nextID
	m.nextID++
	m.tugas = append(m.tugas, t)
	return t, nil
}

func (m *MemoryStore) Get(id int) (Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.tugas {
		if t.ID == id {
			return t, nil
		}
	}
	return Task{}, ErrTugasTidakDitemukan
}

func (m *MemoryStore) List() []Task {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Task, len(m.tugas))
	copy(out, m.tugas)
	return out
}

// Update mengganti Judul dan Selesai tugas ber-ID t.ID (penggantian penuh).
func (m *MemoryStore) Update(t Task) (Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t.Judul = strings.TrimSpace(t.Judul)
	if t.Judul == "" {
		return Task{}, ErrInputKosong
	}
	for i := range m.tugas {
		if m.tugas[i].ID == t.ID {
			m.tugas[i] = t
			return t, nil
		}
	}
	return Task{}, ErrTugasTidakDitemukan
}

func (m *MemoryStore) Delete(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, t := range m.tugas {
		if t.ID == id {
			m.tugas = append(m.tugas[:i], m.tugas[i+1:]...)
			return nil
		}
	}
	return ErrTugasTidakDitemukan
}
