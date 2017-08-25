Golang implementation of DICOM network protocol.

See storeclient and storeserver for example.

Inspired by https://github.com/pydicom/pynetdicom3.

Status as of 2017-08-24:

- storeserver and storeclient sort of work. The server accepts C-STORE requests
  from a remote user and stores dcm files.  The client sends a file to a remote
  server using C-STORE.  I used pynetdicom3 storecu and storecp as peers.

TODO:

- Test compatibility w/ commercial software.
- Better message validation.
- Tighten error handling, e.g., during corrupt messages.
- State machine isn't complete - it misses some transitions.
- Complete the C-STORE client-side impl, and cleanup the code layering structure.
- Remove the "limit" param from the Decoder, and rely on io.EOF detection instead.
