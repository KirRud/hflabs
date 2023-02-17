package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"

	"github.com/gocolly/colly"
	"github.com/tanaikech/go-gdoctableapp"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/jwt"
	"google.golang.org/api/drive/v3"
)

const documentID = "1Mb0WLzd6xh88pyGj4IaPtEaMDjKkFMLOD6SLOLfKtKY"

type RowData struct {
	Code        int
	Description string
}

func parseHTMLTable() (table []RowData) {
	c := colly.NewCollector()

	c.OnHTML("table.confluenceTable tbody", func(h *colly.HTMLElement) {
		h.ForEach("tr", func(_ int, el *colly.HTMLElement) {
			code, _ := strconv.Atoi(el.ChildText("td:nth-child(1)"))
			rowData := RowData{
				Code:        code,
				Description: el.ChildText("td:nth-child(2)"),
			}
			table = append(table, rowData)
		})
	})
	c.Visit("https://confluence.hflabs.ru/pages/viewpage.action?pageId=1181220999")

	log.Println("Data scraped")

	return table
}

func getDifferentRows(table1 []RowData, table2 []RowData) (diffRows []RowData, diffIndx []int) {
	for i, row := range table1 {
		if row != table2[i] {
			diffRows = append(diffRows, row)
			diffIndx = append(diffIndx, i)
		}
	}
	return diffRows, diffIndx
}

func convertRowsTo2DArray(table []RowData) (res [][]interface{}) {
	for _, row := range table {
		res = append(res, []interface{}{row.Code, row.Description})
	}
	return res
}

func convert2DArraytoRows(table [][]string) (res []RowData) {
	for _, row := range table {
		code, _ := strconv.Atoi(row[0])
		res = append(res, RowData{code, row[1]})
	}
	return res
}

func ServiceAccount(secretFile string) *http.Client {
	b, err := ioutil.ReadFile(secretFile)
	if err != nil {
		log.Fatal("Error while reading the credential file", err)
	}
	var s = struct {
		Email      string `json:"client_email"`
		PrivateKey string `json:"private_key"`
	}{}
	json.Unmarshal(b, &s)
	config := &jwt.Config{
		Email:      s.Email,
		PrivateKey: []byte(s.PrivateKey),
		Scopes: []string{
			drive.DriveScope,
		},
		TokenURL: google.JWTTokenURL,
	}
	client := config.Client(context.Background())
	return client
}

func getTable(client *http.Client, documentID string) (*gdoctableapp.Result, error) {
	g := gdoctableapp.New()
	return g.Docs(documentID).GetTables().ShowAPIResponse(true).Do(client)
}

func getTableValues(client *http.Client, documentID string) ([][]string, error) {
	g := gdoctableapp.New()
	tableIndex := 0
	res, err := g.Docs(documentID).TableIndex(tableIndex).GetValues().Do(client)

	return res.Values, err
}

func createTableOnDoc(client *http.Client, table []RowData, documentID string) (*gdoctableapp.Result, error) {
	g := gdoctableapp.New()
	obj := &gdoctableapp.CreateTableRequest{
		Rows:    int64(len(table)),
		Columns: 2,
		Index:   1,
		Values:  convertRowsTo2DArray(table),
	}
	return g.Docs(documentID).CreateTable(obj).Do(client)
}

func updateTableOnDoc(client *http.Client, documentID string, valuesObject []gdoctableapp.ValueObject) (*gdoctableapp.Result, error) {
	g := gdoctableapp.New()
	return g.Docs(documentID).TableIndex(0).SetValuesByObject(valuesObject).Do(client)
}

func createValuesObjectFromRows(rows []RowData, indxs []int) (valuesByObject []gdoctableapp.ValueObject) {

	for i, indx := range indxs {
		vo := &gdoctableapp.ValueObject{}
		vo.Range.StartRowIndex = int64(indx)
		vo.Range.StartColumnIndex = 0
		vo.Values = convertRowsTo2DArray([]RowData{rows[i]})

		valuesByObject = append(valuesByObject, *vo)
	}

	return valuesByObject
}

func writeToDoc(table []RowData) {
	client := ServiceAccount("client_secret.json")

	tbl, err := getTable(client, documentID)
	if err != nil {
		log.Fatal("Could not get table", err)
	}

	if len(tbl.Tables) == 0 {
		_, err := createTableOnDoc(client, table, documentID)

		if err != nil {
			log.Fatal("Could not create file", err)
		}
		log.Println("Table created")

	} else {
		val, err := getTableValues(client, documentID)
		if err != nil {
			log.Fatal("Could no get table value", err)
		}

		tableFromDoc := convert2DArraytoRows(val)
		differentRows, differentIndx := getDifferentRows(table, tableFromDoc)

		if len(differentIndx) != 0 {
			valuesObjects := createValuesObjectFromRows(differentRows, differentIndx)
			_, err := updateTableOnDoc(client, documentID, valuesObjects)

			if err != nil {
				log.Fatal("Could not update file", err)
			}
			log.Println("Table updated")
		}
	}
}

func main() {
	table := parseHTMLTable()
	log.Printf("Table data: %v\n", table)
	writeToDoc(table)
}
