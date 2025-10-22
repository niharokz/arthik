package services

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"time"

	"arthik/config"
	"arthik/models"
)

// LoadRecurrences loads all recurring transactions
func LoadRecurrences() ([]models.Recurrence, error) {
	file, err := os.Open(config.RecurrenceFile)
	if err != nil {
		return []models.Recurrence{}, nil
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	recurrences := []models.Recurrence{}
	for i, record := range records {
		if i == 0 || len(record) < 7 {
			continue
		}
		
		amount, _ := strconv.ParseFloat(record[4], 64)
		nextDate, _ := time.Parse("2006-01-02", record[5])
		dayOfMonth, _ := strconv.Atoi(record[6])

		recurrences = append(recurrences, models.Recurrence{
			ID:          record[0],
			From:        record[1],
			To:          record[2],
			Description: record[3],
			Amount:      amount,
			NextDate:    nextDate,
			DayOfMonth:  dayOfMonth,
		})
	}

	// Sort by next date
	for i := 0; i < len(recurrences); i++ {
		for j := i + 1; j < len(recurrences); j++ {
			if recurrences[j].NextDate.Before(recurrences[i].NextDate) {
				recurrences[i], recurrences[j] = recurrences[j], recurrences[i]
			}
		}
	}

	return recurrences, nil
}

// SaveRecurrence saves or updates a recurring transaction
func SaveRecurrence(rec models.Recurrence) error {
	var records [][]string
	
	if file, err := os.Open(config.RecurrenceFile); err == nil {
		reader := csv.NewReader(file)
		records, _ = reader.ReadAll()
		file.Close()
	} else {
		records = [][]string{{"id", "from", "to", "description", "amount", "nextDate", "dayOfMonth"}}
	}

	found := false
	for i, record := range records {
		if i > 0 && record[0] == rec.ID {
			records[i] = []string{
				rec.ID,
				rec.From,
				rec.To,
				rec.Description,
				fmt.Sprintf("%.2f", rec.Amount),
				rec.NextDate.Format("2006-01-02"),
				strconv.Itoa(rec.DayOfMonth),
			}
			found = true
			break
		}
	}

	if !found {
		records = append(records, []string{
			rec.ID,
			rec.From,
			rec.To,
			rec.Description,
			fmt.Sprintf("%.2f", rec.Amount),
			rec.NextDate.Format("2006-01-02"),
			strconv.Itoa(rec.DayOfMonth),
		})
	}

	file, err := os.Create(config.RecurrenceFile)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()
	
	return writer.WriteAll(records)
}

// DeleteRecurrence deletes a recurring transaction
func DeleteRecurrence(id string) error {
	file, err := os.Open(config.RecurrenceFile)
	if err != nil {
		return err
	}

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	file.Close()
	if err != nil {
		return err
	}

	var newRecords [][]string
	for _, record := range records {
		if len(record) > 0 && record[0] != id {
			newRecords = append(newRecords, record)
		}
	}

	file, err = os.Create(config.RecurrenceFile)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()
	
	return writer.WriteAll(newRecords)
}