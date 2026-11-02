# Sound data

All digital sound is RIFF WAVE, PCM, unsigned 8-bit (`internal/sound`,
test `TestSoundFiles`). There is no MIDI/XMI music: the AIL MIDI code
(`ailxmidi.c`) is linked but the music is streamed WAV; the SAMPLE.* and
*.MDI/*.DIG files are AIL drivers and their instrument banks.

## SOUND.LBX (69 entries)

- Entry 0: a name table, 20-byte records (NUL-padded ASCII), the names
  of entries 1.. in order: ELEBLST2, ELECBLST, ELECNZ2, … BUTTON1 (0x22),
  … The table goes on with longer descriptive names past the 68 used.
- Entries 1..68: WAVE files. 22050 Hz mono (35), 22050 Hz stereo (29),
  10989 Hz mono (2), 11000 Hz stereo (1), 44100 Hz stereo (1).
  `Play_Sound_(n)` plays entry n (`LBX_readRecord_` of the sound file,
  set by `Set_Sound_File_`).

## STREAM.LBX (11 entries), STREAMHD.LBX (21 entries)

The music. WAVE, 22050 Hz stereo 8-bit. Entries that hold only the
22-byte text "cats rule dogs drool\r\n" are placeholders (STREAM 0, 7,
9; STREAMHD 0). `Play_Streaming_Music_` and the other music routines
pick STREAM.LBX (string 0x178bbe) entry n for n ≤ 100, else STREAMHD.LBX
(0x178bb1) entry n − 100 (docs/engine/sound.md). Only the beginning of
the file is read at once; `Sound_Update_` streams the rest in buffers
(`_AIL_load_sample_buffer`), which changes nothing audible.

## Smacker audio

The videos' audio tracks (internal/smk) are not decoded yet.
