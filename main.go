package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
)

type Transacao struct {
	Valor       int       `json:"valor" binding:"required"`
	Tipo        string    `json:"tipo" binding:"required"`
	Descricao   string    `json:"descricao" binding:"required"`
	RealizadaEm time.Time `json:"realizada_em"`
}

type Saldo struct {
	Total       int       `json:"total"`
	DataExtrato time.Time `json:"data_extrato"`
	Limite      int       `json:"limite"`
}

type Extrato struct {
	Saldo      Saldo       `json:"saldo"`
	Transacoes []Transacao `json:"ultimas_transacoes"`
}

var dbpool *pgxpool.Pool

func main() {
	ctx := context.Background()
	var err error
	dbpool, err = pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		dbpool.Close()
		panic(err.Error())
	}
	defer dbpool.Close()

	app := fiber.New()
	app.Post("/clientes/:id/transacoes", transacoes)
	app.Get("/clientes/:id/extrato", extrato)
	app.Listen(":8080")
}

func transacoes(c *fiber.Ctx) error {
	ctx := context.Background()
	clienteId, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.SendStatus(http.StatusNotFound)
	}

	t := &Transacao{}
	err = json.Unmarshal(c.Body(), &t)
	if err != nil {
		return c.SendStatus(http.StatusUnprocessableEntity)
	}

	_, err = dbpool.Exec(ctx, "INSERT INTO transacoes(cliente_id,valor,operacao,descricao)values($1,$2,$3,$4)",
		clienteId, t.Valor, t.Tipo, t.Descricao)
	if err != nil {
		return c.SendStatus(http.StatusUnprocessableEntity)
	}

	fator := 1
	if t.Tipo == "d" {
		fator = -1
	}

	row := dbpool.QueryRow(ctx, `INSERT INTO saldos (cliente_id, limite, balanco) SELECT 
			id, limite,
			((
				SELECT 
					balanco
				FROM saldos 
				WHERE cliente_id = clientes.id
				ORDER BY criado_em DESC
				LIMIT 1
			) + ($2))
		FROM clientes WHERE id = $1 RETURNING limite, balanco
	`, clienteId, (fator * t.Valor))

	limite := 0
	balanco := 0

	err = row.Scan(&limite, &balanco)
	if err == pgx.ErrNoRows {
		return c.SendStatus(http.StatusNotFound)
	}
	if err != nil {
		return c.SendStatus(http.StatusUnprocessableEntity)
	}

	return c.JSON(
		json.RawMessage(fmt.Sprintf(`{"limite": %d,"saldo": %d}`, limite, balanco)),
	)
}

func extrato(c *fiber.Ctx) error {
	clienteId, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.SendStatus(http.StatusNotFound)
	}

	extrato := Extrato{
		Transacoes: make([]Transacao, 0),
	}

	status := http.StatusOK

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		row := dbpool.QueryRow(context.Background(), `
				SELECT 
					limite, 
					now() as data_extrato,
					(
						SELECT 
							balanco
						FROM saldos 
						WHERE cliente_id = clientes.id
						ORDER BY criado_em DESC
						LIMIT 1
					) as balanco
				FROM clientes 
				WHERE id = $1`,
			clienteId,
		)

		err = row.Scan(&extrato.Saldo.Limite, &extrato.Saldo.DataExtrato, &extrato.Saldo.Total)
		if err != nil {
			status = http.StatusNotFound
		}
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		rows, err := dbpool.Query(
			context.Background(),
			`SELECT valor, operacao, descricao, criado_em 
			FROM transacoes WHERE cliente_id = $1
			ORDER BY criado_em DESC LIMIT 10`,
			clienteId,
		)
		if err != nil {
			log.Println(err.Error())
			if status != http.StatusNotFound {
				status = http.StatusBadRequest
			}
			wg.Done()
		}

		for rows.Next() {
			var tr Transacao
			if err := rows.Scan(&tr.Valor, &tr.Tipo, &tr.Descricao, &tr.RealizadaEm); err != nil {
				if status != http.StatusNotFound {
					status = http.StatusBadRequest
				}
			}
			extrato.Transacoes = append(extrato.Transacoes, tr)
		}
		wg.Done()
	}()

	wg.Wait()

	if status == http.StatusOK {
		return c.JSON(extrato)
	} else {
		return c.SendStatus(status)
	}
}
