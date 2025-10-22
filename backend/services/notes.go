package services

import (
	"encoding/csv"
	"os"
	"strings"

	"arthik/config"
	"arthik/models"
	"arthik/utils"
)

// LoadAllNotes loads all notes from all note files
func LoadAllNotes() ([]models.Note, error) {
	files, err := os.ReadDir(config.NotesDir)
	if err != nil {
		return []models.Note{}, nil
	}

	notes := []models.Note{}
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".csv") {
			filePath := config.NotesDir + "/" + file.Name()
			f, err := os.Open(filePath)
			if err != nil {
				continue
			}

			reader := csv.NewReader(f)
			records, _ := reader.ReadAll()
			f.Close()

			for i, record := range records {
				if i == 0 || len(record) < 4 {
					continue
				}
				notes = append(notes, models.Note{
					ID:      record[0],
					Heading: record[1],
					Content: record[2],
					Created: record[3],
				})
			}
		}
	}

	return notes, nil
}

// SaveNote saves or updates a note
func SaveNote(note models.Note) error {
	filename := utils.ToSnakeCase(note.Heading) + ".csv"
	filePath := config.NotesDir + "/" + filename

	var records [][]string
	if file, err := os.Open(filePath); err == nil {
		reader := csv.NewReader(file)
		records, _ = reader.ReadAll()
		file.Close()
	} else {
		records = [][]string{{"id", "heading", "content", "created"}}
	}

	found := false
	for i, record := range records {
		if i > 0 && record[0] == note.ID {
			records[i] = []string{note.ID, note.Heading, note.Content, note.Created}
			found = true
			break
		}
	}

	if !found {
		records = append(records, []string{note.ID, note.Heading, note.Content, note.Created})
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()
	
	return writer.WriteAll(records)
}

// DeleteNote deletes a note
func DeleteNote(id string) error {
	files, err := os.ReadDir(config.NotesDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".csv") {
			filePath := config.NotesDir + "/" + file.Name()
			f, err := os.Open(filePath)
			if err != nil {
				continue
			}

			reader := csv.NewReader(f)
			records, _ := reader.ReadAll()
			f.Close()

			var newRecords [][]string
			deleted := false
			for _, record := range records {
				if len(record) > 0 && record[0] != id {
					newRecords = append(newRecords, record)
				} else if len(record) > 0 && record[0] == id {
					deleted = true
				}
			}

			if deleted {
				if len(newRecords) <= 1 {
					os.Remove(filePath)
				} else {
					file, _ := os.Create(filePath)
					defer file.Close()
					writer := csv.NewWriter(file)
					writer.WriteAll(newRecords)
					writer.Flush()
				}
				break
			}
		}
	}

	return nil
}