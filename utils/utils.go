package utils

import (
	"bytes"
	"context"
	"main/database"

	"github.com/dslipak/pdf"
	"github.com/gofiber/fiber/v2"
	"github.com/jdkato/prose/v2"
)

func SplitAndStore(ctx context.Context, text string, queries *database.Queries, insertedRecord database.Record) error {
	//Sentence package initialization.
	doc, err := prose.NewDocument(text)
	if err != nil {
		return err
	}

	//Store sentences in database.
	for _, sentence := range doc.Sentences() {
		_, err := queries.CreateSentence(ctx, database.CreateSentenceParams{
			Sentence: sentence.Text,
			Pdfid:    insertedRecord.ID,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func SendErrorStatus(c *fiber.Ctx, err string) error {
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"error": err,
	})
}

func SendBadRequestStatus(c *fiber.Ctx, err string) error {
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		"error": err,
	})
}

func ReadPdf(path string) (*pdf.Reader, string, error) {
	r, err := pdf.Open(path)
	if err != nil {
		return nil, "", err
	}
	var buf bytes.Buffer
	b, err := r.GetPlainText()
	if err != nil {
		return nil, "", err
	}
	buf.ReadFrom(b)
	return r, buf.String(), nil
}
