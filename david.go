package main;

import "net/http"
import "io/ioutil"
import "fmt"
import "regexp"
import "sync"
import "os"
import "time"
import "sync/atomic"



type WordSet struct{
    mutex sync.Mutex
    set map[string]bool
}

func NewWordSet() *WordSet{
    return &WordSet{sync.Mutex{}, make(map[string]bool)}
}

func (m *WordSet) add(words []string) []string{
    ret:=make([]string, 0)
    if len(words)==0{
        return ret
    }

    m.mutex.Lock()
    defer m.mutex.Unlock()

    for _,word:=range words{
        if !m.set[word]{
            m.set[word]=true
            ret=append(ret, word)
        }
    }

    return ret
}





type TodoWords struct{
    mutex sync.Mutex
    words []string
}

func NewTodoWords() *TodoWords{
    return &TodoWords{sync.Mutex{}, make([]string, 0, 1024*1024)}
}

func (t *TodoWords) add(words []string){
    if len(words)==0{
        return
    }

    t.mutex.Lock()
    defer t.mutex.Unlock()

    t.words=append(t.words, words...)
}

func (t *TodoWords) pop() string{
    t.mutex.Lock()
    defer t.mutex.Unlock()

    if len(t.words)==0{
        return ""
    }

    ret:=""
    ret, t.words=t.words[len(t.words)-1], t.words[:len(t.words)-1]

    return ret
}

func (t *TodoWords) is_empty_and_there_are_no_working_threads(working_threads *int64) bool{
    t.mutex.Lock()
    defer t.mutex.Unlock()

    return len(t.words)==0 && atomic.LoadInt64(working_threads)==0
}




const NUM_THREADS = 100
func main() {
    r:=regexp.MustCompile("https://www\\.duden\\.de/rechtschreibung/([^\"?#]*)") // [^\"' <>?#]

    word_set:=NewWordSet()
    todo_words:=NewTodoWords()
    word_set.add([]string{"Haus"})
    todo_words.add([]string{"Haus"})

    working_threads:=int64(0)

    for i:=0;i<NUM_THREADS;i++{
        go func(){
            atomic.AddInt64(&working_threads, 1)
            for{
                word:=todo_words.pop()
                if word==""{
                    atomic.AddInt64(&working_threads, -1)
                    time.Sleep(10*time.Second)
                    atomic.AddInt64(&working_threads, 1)
                    continue
                }

                url:="https://www.duden.de/rechtschreibung/"+word
                response,err:=http.Get(url)
                if err!=nil{
                    fmt.Fprintln(os.Stderr, "Could not open", url, "(re-adding to to-do list)")
                    todo_words.add([]string{word})
                    continue
                }

                content, err:=ioutil.ReadAll(response.Body)
                if err!=nil{
                    response.Body.Close()
                    fmt.Fprintln(os.Stderr, "Could not read", url, "(re-adding to to-do list)")
                    todo_words.add([]string{word})
                    continue
                }
                response.Body.Close()

                todo:=make([]string, 20)
                found:=r.FindAllSubmatch(content, -1)
                for _,found:=range found{
                    todo=append(todo, string(found[1]))
                }

                todo=word_set.add(todo)
                todo_words.add(todo)

                fmt.Println(word)
            }
        }()
    }

    for !todo_words.is_empty_and_there_are_no_working_threads(&working_threads){
        time.Sleep(30*time.Second)
    }

    fmt.Fprintln(os.Stderr, "Finished.")
}
