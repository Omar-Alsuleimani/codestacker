package utils

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"main/database"
	"regexp"
	"strconv"
	"strings"

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

func CountFilteredWords(text string) map[string]int {

	trim := strings.TrimSpace(text)
	words := strings.Split(trim, " ")
	stopWords := []string{
		"a", "b", "c", "d", "e", "f", "g", "h", "i", "j",
		"k", "l", "m", "n", "o", "p", "q", "r", "s", "t",
		"u", "v", "w", "x", "y", "z", "about", "above", "after", "again", "against", "all", "am", "an", "and",
		"any", "are", "aren't", "as", "at", "be", "because", "been", "before", "being",
		"below", "between", "both", "but", "by", "can't", "cannot", "could", "couldn't",
		"did", "didn't", "do", "does", "doesn't", "doing", "don't", "down", "during",
		"each", "few", "for", "from", "further", "had", "hadn't", "has", "hasn't", "have",
		"haven't", "having", "he", "he'd", "he'll", "he's", "her", "here", "here's", "hers",
		"herself", "him", "himself", "his", "how", "how's", "i", "i'd", "i'll", "i'm",
		"i've", "if", "in", "into", "is", "isn't", "it", "it's", "its", "itself", "let's",
		"me", "more", "most", "mustn't", "my", "myself", "no", "nor", "not", "of", "off",
		"on", "once", "only", "or", "other", "ought", "our", "ours", "ourselves", "out",
		"over", "own", "same", "shan't", "she", "she'd", "she'll", "she's", "should",
		"shouldn't", "so", "some", "such", "than", "that", "that's", "the", "their", "theirs",
		"them", "themselves", "then", "there", "there's", "these", "they", "they'd", "they'll",
		"they're", "they've", "this", "those", "through", "to", "too", "under", "until", "up",
		"very", "was", "wasn't", "we", "we'd", "we'll", "we're", "we've", "were", "weren't",
		"what", "what's", "when", "when's", "where", "where's", "which", "while", "who",
		"who's", "whom", "why", "why's", "with", "won't", "would", "wouldn't", "you", "you'd",
		"you'll", "you're", "you've", "your", "yours", "yourself", "yourselves", "can",
	}

	filteredWords := map[string]int{}
	reg := regexp.MustCompile("[^a-zA-Z]+")
	for _, word := range words {
		word = reg.ReplaceAllString(word, " ")
		word = strings.TrimSpace(word)
		if strings.Contains(word, " ") {
			for key, value := range CountFilteredWords(word) {
				val := filteredWords[key]
				filteredWords[key] = value + val
			}
			continue
		}
		if word == "" {
			continue
		}

		isStopWord := false
		for _, stopWord := range stopWords {
			if strings.ToLower(word) == stopWord {
				isStopWord = true
				break
			}
		}

		if !isStopWord {
			count := filteredWords[word]
			count++
			filteredWords[word] = count
		}
	}

	return filteredWords
}

func ExtractFrequency(entry string) int {
	if entry == "" {
		return 0
	}

	freq, err := strconv.Atoi(strings.Split(entry, " ")[1])
	if err != nil {
		log.Fatalln(err)
	}
	return freq
}

func ShiftAndUpdateTopFive(index int, key string, value int, topFive *map[int]string) {
	for i := 5; i > index; i-- {
		(*topFive)[i] = (*topFive)[i-1]
	}
	(*topFive)[index] = fmt.Sprintf("%s: %d times", key, value)
}
