Golang implementation of DICOM network protocol.

See storeclient and storeserver for example.

Inspired by https://github.com/pydicom/pynetdicom3.

Status as of 2017-08-31:

- storeserver and storeclient work. The server accepts C-STORE requests
  from a remote user and stores dcm files.  The client sends a file to a remote
  server using C-STORE.  I used pynetdicom3 storecu and storecp as peers.

TODO:

- Implement the rest of DIMSE protocols, such as C-FIND, C-MOVE.
- Test compatibility w/ commercial software.
- Better message validation.
- Remove the "limit" param from the Decoder, and rely on io.EOF detection instead.
