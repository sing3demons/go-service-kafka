package main

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sing3demons/service-go/consume"
	"github.com/sing3demons/service-go/model"
)

func main() {
	// Connect to MongoDB
	db, err := ConnectMonoDB()
	if err != nil {
		log.Fatal(err)
	}

	// ================== Insert data to MongoDB ==================
	kafkaBrokers := []string{"localhost:9092"}
	kafkaTopic := "sales_records"
	consumerGroupID := "my_consumer_group"
	collectionName := "sales_records"

	consume.ConsumeAndInsertToMongoDB(kafkaBrokers, kafkaTopic, consumerGroupID, db.Database("sales_record_db"), collectionName)

	// ================== producer ==================
	sales_records, err := ReadCsv("Sales_Records.csv")
	if err != nil {
		fmt.Println(err)
	}

	// kafkaBrokers := []string{"localhost:9092"}
	// kafkaTopic := "sales_records"

	r := gin.Default()

	r.POST("/sales_records", func(c *gin.Context) {

		if err := produceSalesRecordToKafka(sales_records, kafkaBrokers, kafkaTopic); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
			return
		}
	})

	r.GET("/sales_records", func(c *gin.Context) {
		// ================== Query data from MongoDB ==================
		collection := db.Database("sales_record_db").Collection(collectionName)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		cursor, err := collection.Find(ctx, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
			return
		}

		var sales_records []model.SalesRecord
		for cursor.Next(ctx) {
			var sales_record model.SalesRecord
			err := cursor.Decode(&sales_record)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"message": err.Error(),
				})
				return
			}

			sales_records = append(sales_records, sales_record)
		}

		c.JSON(http.StatusOK, sales_records)
	})

	RunServer(":2566", "SalesRecord-service", r)
}

func ReadCsv(filename string) ([]model.SalesRecord, error) {
	// https://excelbianalytics.com/wp/downloads-18-sample-csv-files-data-sets-for-testing-sales/
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()
	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("error opening file: %v", err)
	}

	if len(records) == 0 {
		return nil, errors.New("no records found in the CSV file")
	}

	sales_records := make([]model.SalesRecord, 0)

	for index, record := range records {
		if index == 0 {
			continue
		}

		sales_record := model.SalesRecord{
			Region:        record[0],
			Country:       record[1],
			ItemType:      record[2],
			SalesChannel:  record[3],
			OrderPriority: record[4],
			OrderDate:     record[5],
			OrderId:       record[6],
			ShipDate:      record[7],
			UnitsSold:     record[8],
			UnitPrice:     record[9],
			UnitCost:      record[10],
			TotalRevenue:  record[11],
			TotalCost:     record[12],
			TotalProfit:   record[13],
		}

		sales_records = append(sales_records, sales_record)
		if index == 100 {
			break
		}
	}

	return sales_records, nil
}

func RunServer(addr, serviceName string, router http.Handler) {
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	go func() {
		fmt.Printf("[%s] http listen: %s\n", serviceName, srv.Addr)

		if err := srv.ListenAndServe(); err != nil && errors.Is(err, http.ErrServerClosed) {
			fmt.Printf("server listen err: %v\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("server forced to shutdown: ", err)
	}

	fmt.Println("server exited")
}
