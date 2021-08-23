package service

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"mime/multipart"

	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"github.com/jszwec/csvutil"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/alex-a-renoire/sigma-homework/model"
	pb "github.com/alex-a-renoire/sigma-homework/pkg/grpcserver/proto"
)

type GRPCPersonService struct {
	remoteStorage pb.StorageServiceClient
}

func NewGRPC(db pb.StorageServiceClient) GRPCPersonService {
	return GRPCPersonService{
		remoteStorage: db,
	}
}

func (s GRPCPersonService) AddPerson(name string) (uuid.UUID, error) {
	resp, err := s.remoteStorage.AddPerson(context.Background(), &pb.AddPersonRequest{
		Name: name,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to add person: %w", err)
	}

	id, err := uuid.Parse(resp.Value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to convert types protobuf to postgres: %w", err)
	}

	return id, nil
}

func (s GRPCPersonService) GetPerson(id uuid.UUID) (model.Person, error) {
	p, err := s.remoteStorage.GetPerson(context.Background(), &pb.UUID{
		Value: id.String(),
	})
	if err != nil {
		if errors.Is(err, redis.Nil) || errors.Is(err, sql.ErrNoRows) {
			return model.Person{}, fmt.Errorf("no such record: %w", err)
		}
		return model.Person{}, fmt.Errorf("failed to get person: %w", err)
	}

	id, err = uuid.Parse(p.Id.Value)
	if err != nil {
		return model.Person{}, fmt.Errorf("failed to convert types protobuf to postgres: %w", err)
	}

	return model.Person{
		Id:   id,
		Name: p.Name,
	}, nil
}

func (s GRPCPersonService) GetAllPersons() ([]model.Person, error) {
	resp, err := s.remoteStorage.GetAllPersons(context.Background(), &emptypb.Empty{})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch persons: %w", err)
	}

	persons := []model.Person{}

	for _, p := range resp.AllPersons {
		id, err := uuid.Parse(p.Id.Value)
		if err != nil {
			return nil, fmt.Errorf("failed to convert types protobuf to postgres: %w", err)
		}

		persons = append(persons, model.Person{
			Id:   id,
			Name: p.Name,
		})
	}

	return persons, nil
}

func (s GRPCPersonService) UpdatePerson(id uuid.UUID, person model.Person) error {
	//Check if there is such a person
	_, err := s.remoteStorage.GetPerson(context.Background(), &pb.UUID{
		Value: id.String(),
	})

	if err != nil {
		if errors.Is(err, redis.Nil) || errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("no such record: %w", err)
		}
		return fmt.Errorf("failed to get person: %w", err)
	}

	_, err = s.remoteStorage.UpdatePerson(context.Background(), &pb.Person{
		Id:   &pb.UUID{Value: id.String()},
		Name: person.Name,
	},
	)

	if err != nil {
		return fmt.Errorf("failed to update person: %w", err)
	}

	return nil
}

func (s GRPCPersonService) DeletePerson(id uuid.UUID) error {
	//Check if there is such a person
	_, err := s.remoteStorage.GetPerson(context.Background(), &pb.UUID{
		Value: id.String(),
	})

	//we assume error is sql.no rows
	if err != nil {
		return fmt.Errorf("there is no such person: %w", err)
	}

	_, err = s.remoteStorage.DeletePerson(context.Background(), &pb.DeletePersonRequest{
		Id: &pb.UUID{Value: id.String()},
	})

	if err != nil {
		return fmt.Errorf("failed to delete person: %w", err)
	}

	return nil
}

///////
//CSV//
///////

func (s GRPCPersonService) ProcessCSV(file multipart.File) error {
	//Parse CSV
	reader := csv.NewReader(file)
	reader.Read()

	//loop of reading
	for i := 0; ; i++ {
		record, err := reader.Read()
		if err != nil {
			if err != io.EOF {
				return fmt.Errorf("Error reading file: %w", err)
			}
			if i == 0 {
				//if there's only headers and no values
				return fmt.Errorf("Malformed csv file: %w", err)
			} else {
				//end of the file
				return nil
			}
		}

		//malformed csv handling
		if len(record) != 2 {
			return fmt.Errorf("Malformed csv file: wrong number of fields")
		}
		if record[0] == "" || record[1] == "" {
			return fmt.Errorf("malformed csv file")
		}
		id, err := uuid.FromBytes([]byte(record[0]))
		if err != nil {
			return fmt.Errorf("malformed id, should be a number: %w", err)
		}

		p := model.Person{
			Id:   id,
			Name: record[1],
		}

		//handle situation when there is such a record and we are updating
		if _, err = s.GetPerson(p.Id); err == nil {
			if err := s.UpdatePerson(p.Id, p); err != nil {
				return fmt.Errorf("failed to update person in db: %w", err)
			}
			return nil
		}

		//If person is not in db, add it with a new id
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, redis.Nil) {
			if _, err = s.AddPerson(p.Name); err != nil {
				return fmt.Errorf("failed to add person to db: %w", err)
			}
		}

		return fmt.Errorf("failed to get person: %w", err)
	}
}

func (s GRPCPersonService) DownloadPersonsCSV() ([]byte, error) {
	//Ask the service to process action
	persons, err := s.GetAllPersons()
	if err != nil {
		return nil, fmt.Errorf("failed to get all persons from db: %w", err)
	}

	//Marshal persons into csv
	ps, err := csvutil.Marshal(persons)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal persons: %w", err)
	}
	return ps, nil
}
