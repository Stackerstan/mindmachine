package eventbucket

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/afjoseph/RAKE.Go"
	"github.com/montanaflynn/stats"
	"mindmachine/mindmachine"
)

type EventBucket struct {
}

func (eb *EventBucket) CalculateMentions() bool {
	calculateMentions()
	return true
}

func (eb *EventBucket) CurrentOrder() []Event {
	return CurrentOrder()
}

func (eb *EventBucket) SingleEvent(e string) Event {
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	if event, ok := currentState.data[e]; ok {
		return event
	}
	return Event{}
}

func (eb *EventBucket) EventList() (e []Event) {
	defer timeTrack(time.Now(), "EventList")
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	i := 0
	for _, event := range currentState.data {
		if i < 10 {
			i++
			e = append(e, event)
		}
	}
	return e
}

type WordCloud struct {
	EventID        string
	EventContent   string
	WordsMentioned map[string]int64
	URLs           map[string]int64
	Keywords       map[string]int64
}

func filterLowSD(input map[string]int64) (output map[string]int64) {
	defer timeTrack(time.Now(), "filterLowSD")
	output = make(map[string]int64)
	var floats []float64
	for _, i := range input {
		floats = append(floats, float64(i))
	}
	a, err := stats.Percentile(floats, 50)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 2)
	} else {
		percentile := int64(a)
		for s, i := range input {
			if i >= percentile {
				output[s] = i
			}
		}
		return output
	}
	return input
}

func filterLowestNumbers(input map[string]int64, threshold int64) (output map[string]int64) {
	defer timeTrack(time.Now(), "filterLowestNumbers")
	output = make(map[string]int64)
	var ints []int64
	for _, i := range input {
		ints = append(ints, i)
	}
	sort.Slice(ints, func(i, j int) bool { return ints[i] < ints[j] })
	var cutoff int64 = 0
	if len(ints) > int(threshold) {
		cutoff = ints[len(ints)-int(threshold)]
	}
	for s, i := range input {
		if i >= cutoff {
			output[s] = i
		}
	}
	return
}

func filterShortest(input map[string]int64, threshold int64) (output map[string]int64) {
	defer timeTrack(time.Now(), "filterShortest")
	output = make(map[string]int64)
	var ints []int
	for s, _ := range input {
		ints = append(ints, len(s))
	}
	sort.Slice(ints, func(i, j int) bool { return ints[i] < ints[j] })
	var cutoff int64 = 0
	if len(ints) > int(threshold) {
		cutoff = int64(ints[len(ints)-int(threshold)])
	}
	for s, i := range input {
		if len(s) >= int(cutoff) {
			output[s] = i
		}
	}
	return
}

func (eb *EventBucket) WordList(max int) (clouds []WordCloud) {
	defer timeTrack(time.Now(), "WordList")
	for _, event := range CurrentOrder() {
		if len(clouds) < max && !IsJSON(event.Event.Content) {
			newCloud := WordCloud{
				EventID:        event.EventID,
				EventContent:   event.Event.Content,
				WordsMentioned: make(map[string]int64),
				URLs:           make(map[string]int64),
				Keywords:       make(map[string]int64),
			}
			for s, _ := range event.MentionMap {
				currentState.mutex.Lock()
				words := appendWords(s)
				for _, s2 := range words {
					if _, err := url.ParseRequestURI(s2); err == nil {
						u, err := url.Parse(s2)
						if err == nil && u.Scheme != "" && u.Host != "" {
							newCloud.URLs[s2]++
						}
					} //else {
					//	newCloud.WordsMentioned[s2]++
					//}
				}
				candidates := appendKeywords(s)
				for _, candidate := range candidates {
					newCloud.Keywords[candidate]++
				}
				currentState.mutex.Unlock()
			}
			//wordsMentioned := filterLowSD(newCloud.WordsMentioned)
			//wordsMentioned := filterLowestNumbers(newCloud.WordsMentioned, 6)
			//newCloud.WordsMentioned = wordsMentioned
			//keywords := filterLowestNumbers(newCloud.Keywords, 6)
			urls := filterLowestNumbers(newCloud.URLs, 6)
			newCloud.URLs = urls
			keywords := filterLowSD(newCloud.Keywords)
			keywords2 := filterShortest(keywords, 6)
			newCloud.Keywords = keywords2
			clouds = append(clouds, newCloud)
		}
	}
	return
}

func appendWords(eventID string) (words []string) {
	defer timeTrack(time.Now(), "appendWords")
	if mention, ok := currentState.data[eventID]; ok {
		if !IsJSON(mention.Event.Content) {
			for _, s2 := range strings.Split(mention.Event.Content, " ") {
				if len(s2) > 3 { //runes := []rune(s2); len(runes) > 3
					s2 = strings.TrimSpace(s2)
					if _, err := url.ParseRequestURI(s2); err == nil {
						words = append(words, s2)
					} else {
						if !regexp.MustCompile(`[^a-zA-Z0-9 ]+`).MatchString(s2) {
							if len(s2) < 20 {
								words = append(words, s2)
							}
						}
					}

				}
			}
			for mID, _ := range mention.MentionMap {
				words = append(words, appendWords(mID)...)
			}
		}
	}
	return
}

func appendKeywords(eventID string) (keywords []string) {
	defer timeTrack(time.Now(), "appendKeywords")
	if mention, ok := currentState.data[eventID]; ok {
		if !IsJSON(mention.Event.Content) {
			candidates := rake.RunRake(mention.Event.Content)
			for _, candidate := range candidates {
				//fmt.Printf("%s --> %f\n", candidate.Key, candidate.Value)
				if candidate.Value > 4 {
					if len(candidate.Key) < 50 {
						keywords = append(keywords, candidate.Key)
						if candidate.Key == "show show share paper airplane" {
							fmt.Println(eventID)
						}
					}
				}
			}
			for mID, _ := range mention.MentionMap {
				keywords = append(keywords, appendKeywords(mID)...)
			}
		}
	}
	return
}

func IsJSON(str string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(str), &js) == nil
}

func timeTrack(start time.Time, name string) {
	//elapsed := time.Since(start)
	//mindmachine.LogCLI(fmt.Sprintf("%s took %s", name, elapsed), 4)
}
