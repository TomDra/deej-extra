const NUM_SLIDERS = 5;
const analogInputs = [A3, A2, A1, A0, A10];

const NUM_BUTTONS = 6;
const buttonInputs = [D9, D8, D7, D6, D5, D4]; 

let analogSliderValues = new Array(NUM_SLIDERS).fill(0);
let buttonValues = new Array(NUM_BUTTONS).fill(0);

// Setup: configure pins
function setup() {
  // Analog pins don't need pinMode in Espruino (they default to analog input)
  // Buttons need pull-up resistors
  buttonInputs.forEach(pin => pinMode(pin, 'input_pullup'));
}

// Read slider + button values
function updateValues() {
  for (let i = 0; i < NUM_SLIDERS; i++) {
    analogSliderValues[i] = analogRead(analogInputs[i]); 
    // analogRead returns 0.0–1.0 in Espruino; convert to 0–1023 like Arduino
    analogSliderValues[i] = Math.round(analogSliderValues[i] * 1023);
  }

  for (let i = 0; i < NUM_BUTTONS; i++) {
    buttonValues[i] = digitalRead(buttonInputs[i]);
  }
}

// Build and send the data string over Serial
function sendValues() {
  let builtString = "";

  for (let i = 0; i < NUM_SLIDERS; i++) {
    builtString += "s" + analogSliderValues[i];
    if (i < NUM_SLIDERS - 1) builtString += "|";
  }

  if (NUM_BUTTONS > 0) builtString += "|";

  for (let i = 0; i < NUM_BUTTONS; i++) {
    builtString += "b" + buttonValues[i];
    if (i < NUM_BUTTONS - 1) builtString += "|";
  }

  console.log(builtString);
}

// (Optional) Debug print for sliders
function printSliderValues() {
  let out = "";
  for (let i = 0; i < NUM_SLIDERS; i++) {
    out += "Slider #" + (i+1) + ": " + analogSliderValues[i] + " mV";
    if (i < NUM_SLIDERS - 1) out += " | ";
  }
  console.log(out);
}

// Setup once
setup();

// Run every 10 ms (similar to Arduino loop with delay(10))
setInterval(() => {
  updateValues();
  sendValues(); // send data continuously
  // printSliderValues(); // uncomment for debugging
}, 10);
