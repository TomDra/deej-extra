@echo off
setlocal

REM INSTALL nircmd 
REM RUN `nircmd showsounddevices` to get all audio devices to get the names

REM === List of audio devices ===
set DEVICE1=Speakers
set DEVICE2=Headset Earphone

REM === Path to state file ===
set STATE_FILE=%~dp0current_device.txt

REM === Read current state from file (if it exists) ===
if exist "%STATE_FILE%" (
    set /p CURRENT=<"%STATE_FILE%"
) else (
    REM Default to DEVICE1 if no state file
    set CURRENT=%DEVICE1%
)

REM === Toggle between devices ===
if "%CURRENT%"=="%DEVICE1%" (
    echo Switching to %DEVICE2%
    nircmd setdefaultsounddevice "%DEVICE2%"
    echo %DEVICE2%>"%STATE_FILE%"
) else (
    echo Switching to %DEVICE1%
    nircmd setdefaultsounddevice "%DEVICE1%"
    echo %DEVICE1%>"%STATE_FILE%"
)

endlocal
