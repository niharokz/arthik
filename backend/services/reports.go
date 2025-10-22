package services

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"time"

	"arthik/config"
	"arthik/models"
	"arthik/utils"
)

// LoadMonthlyReports loads all monthly reports
func LoadMonthlyReports() ([]models.MonthlyReport, error) {
	file, err := os.Open(config.ReportsFile)
	if err != nil {
		return []models.MonthlyReport{}, nil
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	reports := []models.MonthlyReport{}
	for i, record := range records {
		if i == 0 || len(record) < 4 {
			continue
		}
		
		netWorth, _ := strconv.ParseFloat(record[1], 64)
		liabilities, _ := strconv.ParseFloat(record[2], 64)
		savings, _ := strconv.ParseFloat(record[3], 64)

		reports = append(reports, models.MonthlyReport{
			Date:        record[0],
			NetWorth:    netWorth,
			Liabilities: liabilities,
			Savings:     savings,
		})
	}

	return reports, nil
}

// SaveMonthlyReport saves or updates a monthly report
func SaveMonthlyReport(report models.MonthlyReport) error {
	var records [][]string
	
	if file, err := os.Open(config.ReportsFile); err == nil {
		reader := csv.NewReader(file)
		records, _ = reader.ReadAll()
		file.Close()
	} else {
		records = [][]string{{"date", "netWorth", "liabilities", "savings"}}
	}

	found := false
	for i, record := range records {
		if i > 0 && record[0] == report.Date {
			records[i] = []string{
				report.Date,
				fmt.Sprintf("%.2f", report.NetWorth),
				fmt.Sprintf("%.2f", report.Liabilities),
				fmt.Sprintf("%.2f", report.Savings),
			}
			found = true
			break
		}
	}

	if !found {
		records = append(records, []string{
			report.Date,
			fmt.Sprintf("%.2f", report.NetWorth),
			fmt.Sprintf("%.2f", report.Liabilities),
			fmt.Sprintf("%.2f", report.Savings),
		})
	}

	file, err := os.Create(config.ReportsFile)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()
	
	return writer.WriteAll(records)
}

// UpdateMonthlyReport updates the monthly report for a given transaction date
func UpdateMonthlyReport(transactionDate time.Time) error {
	month := utils.GetMonthFromDate(transactionDate)
	transactions, err := LoadTransactions(month)
	if err != nil {
		return err
	}

	// Calculate net worth
	netWorth := CalculateNetWorth()

	// Calculate liabilities
	var totalLiabilities float64
	for _, acc := range accounts {
		if acc.IncludeInNetWorth && acc.Category == "Liabilities" {
			totalLiabilities += acc.CurrentBalance
		}
	}

	// Calculate income and expenses for the month
	var monthIncome, monthExpenses float64
	for _, t := range transactions {
		fromCat, toCat := getAccountCategories(t.From, t.To)

		if fromCat == "Revenue" {
			monthIncome += t.Amount
		}
		if toCat == "Expenses" {
			monthExpenses += t.Amount
		}
	}

	// Calculate total budget
	var totalBudget float64
	for _, acc := range accounts {
		if acc.Category == "Expenses" && acc.Budget > 0 {
			totalBudget += acc.Budget
		}
	}

	savings := totalBudget - monthExpenses

	// Get last day of the month for the date
	year, _ := strconv.Atoi(month[:4])
	monthNum, _ := strconv.Atoi(month[4:])
	lastDay := time.Date(year, time.Month(monthNum+1), 0, 0, 0, 0, 0, time.Local)
	dateStr := lastDay.Format("2006-01-02")

	return SaveMonthlyReport(models.MonthlyReport{
		Date:        dateStr,
		NetWorth:    netWorth,
		Liabilities: totalLiabilities,
		Savings:     savings,
	})
}

// GetReportByMonth retrieves a monthly report for a specific month
func GetReportByMonth(month string) (*models.MonthlyReport, error) {
	reports, err := LoadMonthlyReports()
	if err != nil {
		return nil, err
	}

	for _, report := range reports {
		if report.Date[:7] == month[:7] { // Compare YYYY-MM
			return &report, nil
		}
	}

	return nil, fmt.Errorf("report not found for month %s", month)
}

// GetRecentReports returns the most recent N reports
func GetRecentReports(count int) ([]models.MonthlyReport, error) {
	reports, err := LoadMonthlyReports()
	if err != nil {
		return nil, err
	}

	if len(reports) <= count {
		return reports, nil
	}

	return reports[len(reports)-count:], nil
}
