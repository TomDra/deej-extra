package deej

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
	"os"

	"github.com/jacobsa/go-serial/serial"
	"go.uber.org/zap"

	"github.com/omriharel/deej/pkg/deej/util"
)

// SerialIO provides a deej-aware abstraction layer to managing serial I/O
type SerialIO struct {
	deej   *Deej
	logger *zap.SugaredLogger

	stopChannel chan bool
	connected   bool
	connOptions serial.OpenOptions
	conn        io.ReadWriteCloser

	lastKnownNumSliders        int
	currentSliderPercentValues []float32
	lastKnownNumButtons        int
	currentButtonValues 			 []int

	sliderMoveConsumers []chan SliderMoveEvent
	buttonEventConsumers []chan ButtonEvent
}

// SliderMoveEvent represents a single slider move captured by deej
type SliderMoveEvent struct {
	SliderID     int
	PercentValue float32
}

// SliderMoveEvent represents a single slider move captured by deej
type ButtonEvent struct {
	ButtonID     int
	Value 			 int
}

var expectedLinePattern = regexp.MustCompile(`^\w{1}\d{1,4}(\|\w{1}\d{1,4})*\r\n$`)

// NewSerialIO creates a SerialIO instance that uses the provided deej
// instance's connection info to establish communications with the arduino chip
func NewSerialIO(deej *Deej, logger *zap.SugaredLogger) (*SerialIO, error) {
	logger = logger.Named("serial")

	sio := &SerialIO{
		deej:                deej,
		logger:              logger,
		stopChannel:         make(chan bool),
		connected:           false,
		conn:                nil,
		sliderMoveConsumers: []chan SliderMoveEvent{},
		buttonEventConsumers: []chan ButtonEvent{},
	}

	logger.Debug("Created serial i/o instance")

	// respond to config changes
	sio.setupOnConfigReload()

	return sio, nil
}

// Start attempts to connect to our arduino chip with retries for a set time
func (sio *SerialIO) Start() error {
    // don't allow multiple concurrent connections
    if sio.connected {
        sio.logger.Warn("Already connected, can't start another without closing first")
        return errors.New("serial: connection already active")
    }

    // set minimum read size according to platform (0 for windows, 1 for linux)
    minimumReadSize := 0
    if util.Linux() {
        minimumReadSize = 1
    }

    sio.connOptions = serial.OpenOptions{
        PortName:        sio.deej.config.ConnectionInfo.COMPort,
        BaudRate:        uint(sio.deej.config.ConnectionInfo.BaudRate),
        DataBits:        8,
        StopBits:        1,
        MinimumReadSize: uint(minimumReadSize),
    }

    retryDuration := time.Duration(sio.deej.config.ConnectionInfo.RetryDuration) * time.Second
    deadline := time.Now().Add(retryDuration)

    sio.logger.Debugw("Attempting serial connection with retries",
        "comPort", sio.connOptions.PortName,
        "baudRate", sio.connOptions.BaudRate,
        "retryDuration", retryDuration,
    )

    var err error
    for {
        sio.conn, err = serial.Open(sio.connOptions)
        if err == nil {
            break // success
        }

        sio.logger.Warnw("Failed to open serial connection, retrying...",
            "error", err,
            "remaining", time.Until(deadline),
        )

        if time.Now().After(deadline) {
            return fmt.Errorf("failed to connect to %s within %s: %w",
                sio.connOptions.PortName, retryDuration, err)
        }

        time.Sleep(2 * time.Second) // wait before retrying
    }

    // Connected
    namedLogger := sio.logger.Named(strings.ToLower(sio.connOptions.PortName))
    namedLogger.Infow("Connected", "conn", sio.conn)
    sio.connected = true

    // read lines or await a stop
    go func() {
        connReader := bufio.NewReader(sio.conn)
        lineChannel := sio.readLine(namedLogger, connReader)

        for {
            select {
            case <-sio.stopChannel:
                sio.close(namedLogger)
            case line := <-lineChannel:
                sio.handleLine(namedLogger, line)
            }
        }
    }()

    return nil
}

// Stop signals us to shut down our serial connection, if one is active
func (sio *SerialIO) Stop() {
	if sio.connected {
		sio.logger.Debug("Shutting down serial connection")
		sio.stopChannel <- true
	} else {
		sio.logger.Debug("Not currently connected, nothing to stop")
	}
}

// SubscribeToSliderMoveEvents returns an unbuffered channel that receives
// a sliderMoveEvent struct every time a slider moves
func (sio *SerialIO) SubscribeToSliderMoveEvents() chan SliderMoveEvent {
	ch := make(chan SliderMoveEvent)
	sio.sliderMoveConsumers = append(sio.sliderMoveConsumers, ch)

	return ch
}

func (sio *SerialIO) SubscribeToButtonEvents() chan ButtonEvent {
	ch := make(chan ButtonEvent)
	sio.buttonEventConsumers = append(sio.buttonEventConsumers, ch)

	return ch
}

func (sio *SerialIO) setupOnConfigReload() {
	configReloadedChannel := sio.deej.config.SubscribeToChanges()

	const stopDelay = 50 * time.Millisecond

	go func() {
		for {
			select {
			case <-configReloadedChannel:

				// make any config reload unset our slider number to ensure process volumes are being re-set
				// (the next read line will emit SliderMoveEvent instances for all sliders)\
				// this needs to happen after a small delay, because the session map will also re-acquire sessions
				// whenever the config file is reloaded, and we don't want it to receive these move events while the map
				// is still cleared. this is kind of ugly, but shouldn't cause any issues
				go func() {
					<-time.After(stopDelay)
					sio.lastKnownNumSliders = 0
				}()

				// if connection params have changed, attempt to stop and start the connection
				if sio.deej.config.ConnectionInfo.COMPort != sio.connOptions.PortName ||
					uint(sio.deej.config.ConnectionInfo.BaudRate) != sio.connOptions.BaudRate {

					sio.logger.Info("Detected change in connection parameters, attempting to renew connection")
					sio.Stop()

					// let the connection close
					<-time.After(stopDelay)

					if err := sio.Start(); err != nil {
						sio.logger.Warnw("Failed to renew connection after parameter change", "error", err)
					} else {
						sio.logger.Debug("Renewed connection successfully")
					}
				}
			}
		}
	}()
}

func (sio *SerialIO) close(logger *zap.SugaredLogger) {
	if err := sio.conn.Close(); err != nil {
		logger.Warnw("Failed to close serial connection", "error", err)
	} else {
		logger.Debug("Serial connection closed")
	}

	sio.conn = nil
	sio.connected = false
}

func (sio *SerialIO) readLine(logger *zap.SugaredLogger, reader *bufio.Reader) chan string {
	ch := make(chan string)

	go func() {
		for {
			line, err := reader.ReadString('\n')
			if err != nil {

				if sio.deej.Verbose() {
					logger.Warnw("Read error from serial (likely disconnected)", "error", err, "line", line)
				}

				// trigger reconnect attempt
                go sio.reconnect(logger)
				return
			}

			if sio.deej.Verbose() {
				logger.Debugw("Read new line", "line", line)
			}

			// deliver the line to the channel
			ch <- line
		}
	}()

	return ch
}

func (sio *SerialIO) handleLine(logger *zap.SugaredLogger, line string) {

	// this function receives an unsanitized line which is guaranteed to end with LF,
	// but most lines will end with CRLF. it may also have garbage instead of
	// deej-formatted values, so we must check for that! just ignore bad ones
	if !expectedLinePattern.MatchString(line) {
		return
	}

	// trim the suffix
	line = strings.TrimSuffix(line, "\r\n")

	// split on pipe (|), this gives a slice of numerical strings between "0" and "1023"
	splitLine := strings.Split(line, "|")

	splitLineSliders := []string{}
	splitLineButtons := []string{}

	for _, splitValue := range splitLine {
		if splitValue[0] == 's' {
			splitLineSliders = append(splitLineSliders, strings.Replace(splitValue, "s", "", -1))
		}else if splitValue[0] == 'b' {
			splitLineButtons = append(splitLineButtons, strings.Replace(splitValue, "b", "", -1))
		}
	}

	numSliders := len(splitLineSliders)
	numButtons := len(splitLineButtons)

	// update our slider count, if needed - this will send slider move events for all
	if numSliders != sio.lastKnownNumSliders {
		logger.Infow("Detected sliders", "amount", numSliders)
		sio.lastKnownNumSliders = numSliders
		sio.currentSliderPercentValues = make([]float32, numSliders)

		// reset everything to be an impossible value to force the slider move event later
		for idx := range sio.currentSliderPercentValues {
			sio.currentSliderPercentValues[idx] = -1.0
		}
	}

	if numButtons != sio.lastKnownNumButtons {
		logger.Infow("Detected buttons", "amount", numButtons)
		sio.lastKnownNumButtons = numButtons
		sio.currentButtonValues = make([]int, numButtons)

		// reset everything to be an impossible value to force the slider move event later
		for idx := range sio.currentButtonValues {
			sio.currentButtonValues[idx] = -1.0
		}
	}

	// for each slider:
	moveEvents := []SliderMoveEvent{}
	for sliderIdx, stringValue := range splitLineSliders {

		// logger.Debug(stringValue);


			// convert string values to integers ("1023" -> 1023)
			number, _ := strconv.Atoi(stringValue)

			// turns out the first line could come out dirty sometimes (i.e. "4558|925|41|643|220")
			// so let's check the first number for correctness just in case
			if sliderIdx == 0 && number > 1023 {
				sio.logger.Debugw("Got malformed line from serial, ignoring", "line", line)
				return
			}

			// map the value from raw to a "dirty" float between 0 and 1 (e.g. 0.15451...)
			dirtyFloat := float32(number) / 1023.0

			// normalize it to an actual volume scalar between 0.0 and 1.0 with 2 points of precision
			normalizedScalar := util.NormalizeScalar(dirtyFloat)

			// if sliders are inverted, take the complement of 1.0
			if sio.deej.config.InvertSliders {
				normalizedScalar = 1 - normalizedScalar
			}

			// check if it changes the desired state (could just be a jumpy raw slider value)
			if util.SignificantlyDifferent(sio.currentSliderPercentValues[sliderIdx], normalizedScalar, sio.deej.config.NoiseReductionLevel) {

				// if it does, update the saved value and create a move event
				sio.currentSliderPercentValues[sliderIdx] = normalizedScalar

				moveEvents = append(moveEvents, SliderMoveEvent{
					SliderID:     sliderIdx,
					PercentValue: normalizedScalar,
				})

				if sio.deej.Verbose() {
					logger.Debugw("Slider moved", "event", moveEvents[len(moveEvents)-1])
				}
			}


	}

	buttonEvents := []ButtonEvent{}
	for buttonId, stringValue := range splitLineButtons {

		
			//button handler
			stringValue = strings.Replace(stringValue, "b", "", -1)
			number, _ := strconv.Atoi(stringValue)
			
			if sio.currentButtonValues[buttonId] != number {

				sio.currentButtonValues[buttonId] = number

				buttonEvents = append(buttonEvents, ButtonEvent{
					ButtonID:     buttonId,
					Value: 				number,
				})

				if sio.deej.Verbose() {
					logger.Debugw("Button changed", "event", buttonEvents[len(buttonEvents)-1])
				}

			}


	}

	// deliver move events if there are any, towards all potential consumers
	if len(moveEvents) > 0 {
		for _, consumer := range sio.sliderMoveConsumers {
			for _, moveEvent := range moveEvents {
				consumer <- moveEvent
			}
		}
	}

	if len(buttonEvents) > 0 {
		for _, consumer := range sio.buttonEventConsumers {
			for _, buttonEvent := range buttonEvents {
				consumer <- buttonEvent
			}
		}
	}
}

func (sio *SerialIO) reconnect(logger *zap.SugaredLogger) {
    sio.logger.Warn("Serial connection lost, attempting to reconnect...")

    // Close the existing connection safely
    sio.close(logger)

    retryDuration := time.Duration(sio.deej.config.ConnectionInfo.RetryDuration) * time.Second
    deadline := time.Now().Add(retryDuration)

    for {
        conn, err := serial.Open(sio.connOptions)
        if err == nil {
            sio.conn = conn
            sio.connected = true
            logger.Infow("Reconnected successfully", "port", sio.connOptions.PortName)

            // restart read goroutine
            go func() {
                connReader := bufio.NewReader(sio.conn)
                lineChannel := sio.readLine(logger, connReader)
                for {
                    select {
                    case <-sio.stopChannel:
                        sio.close(logger)
                        return
                    case line := <-lineChannel:
                        sio.handleLine(logger, line)
                    }
                }
            }()
            return
        }

        if time.Now().After(deadline) {
            logger.Errorw("Failed to reconnect within retry duration", "error", err)
			logger.Error("Exiting program because serial device could not be reconnected")
            os.Exit(1)
            return
        }

        time.Sleep(2 * time.Second)
    }
}



