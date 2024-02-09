package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func main() {
	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		panic(err.Error())
	}
	defer db.Close()

	r := gin.Default()
	r.POST("/clientes/:id/transacoes", transacoes)
	r.GET("/clientes/:id/extrato", extrato)
	r.Run()
}

type Transacao struct {
	Valor       int       `json:"valor":"required"`
	Tipo        string    `json:"tipo":"required"`
	Descricao   string    `json:"descricao":"required"`
	RealizadaEm time.Time `json:"realizada_em":"required"`
}

type Saldo struct {
	Total       int       `json:"total"`
	DataExtrato time.Time `json:"data_extrato"`
	Limite      int       `json:"limite"`
}

type Extrato struct {
	Saldo Saldo `json:"saldo"`
}

func (t *Transacao) estaValido() bool {
	return validaDescricao(t.Descricao) && validaTipo(t.Tipo)
}

func transacoes(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	fmt.Println(id)

	var t Transacao
	if c.BindJSON(&t) != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	fmt.Println("descricao", t.Descricao)

	if !t.estaValido() {
		c.Status(http.StatusBadRequest)
		return
	}

	jsonParam := json.RawMessage(fmt.Sprintf(`{"limite":"%d","saldo":"%d"}`, 1, 2))

	c.PureJSON(http.StatusOK, jsonParam)
}

func extrato(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": 1,
	})

	// {
	// 	"saldo": {
	// 	  "total": -9098,
	// 	  "data_extrato": "2024-01-17T02:34:41.217753Z",
	// 	  "limite": 100000
	// 	},
	// 	"ultimas_transacoes": [
	// 	  {
	// 		"valor": 10,
	// 		"tipo": "c",
	// 		"descricao": "descricao",
	// 		"realizada_em": "2024-01-17T02:34:38.543030Z"
	// 	  },
	// 	  {
	// 		"valor": 90000,
	// 		"tipo": "d",
	// 		"descricao": "descricao",
	// 		"realizada_em": "2024-01-17T02:34:38.543030Z"
	// 	  }
	// 	]
	//   }
}

func validaTipo(param string) bool {
	return param == "c" || param == "d"
}

func validaDescricao(param string) bool {
	return len(param) >= 1 && len(param) <= 10
}
