package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/costexplorer"
	"github.com/olekukonko/tablewriter"
)

// getDates returns a DateInterval for the last week
func getDates() *costexplorer.DateInterval {
	now := time.Now()
	end := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	start := end.AddDate(0, -12, 0)
	dateRange := costexplorer.DateInterval{}
	dateRange.SetEnd(end.Format("2006-01-02"))
	dateRange.SetStart(start.Format("2006-01-02"))
	return &dateRange
}

// covert string to float to string for formatting
func formatNumber(s string) string {
	f, _ := strconv.ParseFloat(s, 64)
	// fmt.Printf("%.2f", f)
	f = f / 100
	return fmt.Sprintf("%.2f", f)
}

// generate the date headers for the table
func dateHeaders() []string {
	now := time.Now()
	dates := []string{"AWS Service"}
	for i := 7; i > 0; i-- {
		n := now.AddDate(0, 0, -i)
		dates = append(dates, n.Format("01-02"))
	}
	return dates
}

func getCostMapping() map[string]string {
	file, err := os.Open("costs.csv")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	var line []string

	m := make(map[string]string)
	for {
		line, err = reader.Read()
		if err != nil {
			break
		}
		m[line[0]] = line[1]
	}
	return m
}

func main() {
	var (
		profile = flag.String("profile", "dev", "profile flag")
	)
	flag.Parse()
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		Config:  aws.Config{Region: aws.String("us-northeast-1")},
		Profile: *profile,
	}))

	svc := costexplorer.New(sess)
	dates := getDates()
	var servicePtrs []*string
	var service []string = []string{"Amazon Elastic Compute Cloud - Compute"}
	servicePtrs = aws.StringSlice(service)
	var userTypePtrs []*string
	var userType []string = []string{"EC2: Running Hours"}
	userTypePtrs = aws.StringSlice(userType)
	// var rauPtrs []*string
	// var rau []string = []string{"Standard Reserved Instances"}
	// rauPtrs = aws.StringSlice(rau)
	resp, err := svc.GetCostAndUsage((&costexplorer.GetCostAndUsageInput{
		Metrics:     []*string{aws.String("UnblendedCost"), aws.String("UsageQuantity")},
		TimePeriod:  dates,
		Granularity: aws.String("MONTHLY"),
		GroupBy: []*costexplorer.GroupDefinition{
			&costexplorer.GroupDefinition{
				Key:  aws.String("INSTANCE_TYPE"),
				Type: aws.String("DIMENSION"),
			},
		},
		Filter: &costexplorer.Expression{
			And: []*costexplorer.Expression{
				&costexplorer.Expression{
					Dimensions: &costexplorer.DimensionValues{
						Key:    aws.String("SERVICE"),
						Values: servicePtrs,
					},
				},
				&costexplorer.Expression{
					Dimensions: &costexplorer.DimensionValues{
						Key:    aws.String("USAGE_TYPE_GROUP"),
						Values: userTypePtrs,
					},
				},
				// &costexplorer.Expression{
				// 	Dimensions: &costexplorer.DimensionValues{
				// 		Key:    aws.String("PURCHASE_TYPE"),
				// 		Values: rauPtrs,
				// 	},
				// },
			},
		},
	}))

	if err != nil {
		fmt.Println(err)
	} else {
		// fmt.Println(resp)
	}
	costMapping := getCostMapping()
	for _, group := range resp.ResultsByTime {
		period := tablewriter.NewWriter(os.Stdout)
		period.SetHeader([]string{"Start", "End"})
		period.Append([]string{*group.TimePeriod.Start, *group.TimePeriod.End})
		period.Render()
		// fmt.Println("Time Period", group.TimePeriod)
		// fmt.Println(group.Groups)
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Instance Type", "Hours", "Cost"})
		sum := 0.0
		for _, key := range group.Groups {
			// fmt.Println(*key.Keys[0])
			// fmt.Println(*key.Metrics["UsageQuantity"].Amount, "Hours")
			hours, err := strconv.ParseFloat(*key.Metrics["UsageQuantity"].Amount, 32)
			if err != nil {
				fmt.Println(err)
			}
			costPerHour, err := strconv.ParseFloat(costMapping[*key.Keys[0]], 32)
			if err != nil {
				fmt.Println(err)
			}
			cost := costPerHour * hours
			unblendedCost, err := strconv.ParseInt(*key.Metrics["UnblendedCost"].Amount, 10, 32)
			if unblendedCost == 0 {
				table.Append([]string{*key.Keys[0], *key.Metrics["UsageQuantity"].Amount, strconv.FormatFloat(cost, 'f', 4, 32)})
				sum += cost
			} else {
				table.Append([]string{*key.Keys[0], *key.Metrics["UsageQuantity"].Amount, *key.Metrics["UnblendedCost"].Amount})
				sum += float64(unblendedCost)
				// fmt.Println(cost, "USD", "\n\n")
			}
		}
		table.SetFooter([]string{"", "TOTAL", strconv.FormatFloat(sum, 'f', 4, 32)})
		table.Render() // Send output
	}
}
