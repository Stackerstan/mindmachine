package mindmachine

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"time"

	"github.com/mborders/logmatic"
)

// Logs to the terminal. Level options are: 0 fatal error (stack dump), 1 serious error (stack dump), 2 warning, 3 debug, 4 info, 5 trace (stack dump).
func LogCLI(message interface{}, level int) {
	l := logmatic.NewLogger()
	l.SetLevel(logmatic.TRACE)
	l.ExitOnFatal = true
	message = fmt.Sprint(message)
	switch level {
	case 5:
		debug.PrintStack()
		l.Trace("%v", message)
	case 4:
		l.Info("%v", message)
	case 3:
		l.Debug("%v", message)
	case 2:
		l.Warn("%v", message)
	case 1:
		debug.PrintStack()
		l.Error("%v", message)
	case 0:
		debug.PrintStack()
		l.Error("%v", message)
		Shutdown()
	}

}

// Actor logs to file so that messages can be easily traced throughout the system as they pass from actor to actor
func LogMind(log MindLog) bool {
	if MakeOrGetConfig().GetBool("logActors") {
		entry := time.Now().String() + fmt.Sprintf("%#v", log) + "\n\n"
		f, err := os.OpenFile(MakeOrGetConfig().GetString("rootDir")+"/actorMessages.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			LogCLI(err, 0)
		}
		defer f.Close()
		_, err = io.WriteString(f, entry)
		if err != nil {
			return false
		}
	}
	return true
}
