package redisstorage

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/alex-a-renoire/sigma-homework/model"
	"github.com/go-redis/redis"
)

type RDSdb struct {
	currentPersonId int
	Client          *redis.Client
}

func NewRDS(addr string, pwd string, dbname int) *RDSdb {
	return &RDSdb{
		currentPersonId: 0,
		Client: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: pwd,
			DB:       dbname,
		}),
	}
}

func (db *RDSdb) AddPerson(p model.Person) (int, error) {
	db.currentPersonId += 1

	person, err := json.Marshal(p)
	if err != nil {
		return 0, fmt.Errorf("Cannot add person to db: %w", err)
	}

	_, err = db.Client.Set("person:"+strconv.Itoa(db.currentPersonId), person, 0).Result()
	if err != nil {
		return 0, fmt.Errorf("Cannot add person to db: %w", err)
	}

	return db.currentPersonId, nil
}

func (db *RDSdb) GetPerson(id int) (model.Person, error) {
	var person model.Person

	res, err := db.Client.Get("person:" + strconv.Itoa(id)).Result()
	if err != nil {
		return model.Person{}, fmt.Errorf("failed to find person: %w", err)
	}

	if err = json.Unmarshal([]byte(res), &person); err != nil {
		return model.Person{}, fmt.Errorf("Cannot retrieve person from db: %w", err)
	}

	person.Id = id

	return person, nil
}

func (db *RDSdb) GetAllPersons() ([]model.Person, error) {
	var persons []model.Person

	keys, err := db.Client.Keys("person:*").Result()
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve persons from db: %w", err)
	}

	for _, k := range keys {
		var person model.Person
		reply, err := db.Client.Get(k).Result()
		if err != nil {
			return nil, fmt.Errorf("Failed to retrieve persons from db: %w", err)
		}

		if err := json.Unmarshal([]byte(reply), &person); err != nil {
			return nil, fmt.Errorf("Failed to retrieve persons from db: %w", err)
		}

		person.Id, err = strconv.Atoi(strings.TrimPrefix(k, "person:"))
		if err != nil {
			return nil, fmt.Errorf("malformed id or prefix: %w", err)
		}

		persons = append(persons, person)
	}

	return persons, nil
}

func (db *RDSdb) UpdatePerson(id int, p model.Person) error {
	log.Printf("Id: %d", id)

	person, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("Cannot marshal person: %w", err)
	}

	_, err = db.Client.Set("person:"+strconv.Itoa(id), person, 0).Result()
	if err != nil {
		return fmt.Errorf("failed to update record: %w", err)
	}

	return nil
}

func (db *RDSdb) DeletePerson(id int) error {
	_, err := db.Client.Del("person:" + strconv.Itoa(id)).Result()
	if err != nil {
		return fmt.Errorf("failed to delete person: %w", err)
	}
	return nil
}
