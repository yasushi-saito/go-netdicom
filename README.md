Golang implementation of DICOM network protocol.

See storeclient and storeserver for example.

Inspired by https://github.com/pydicom/pynetdicom3.

Status as of 2017-08-23:

- storeserver sort of works. It accepts C-STORE requests from a remote user and stores dcm files.
  I tested using pynetdicom3 storecu.

- storeclient is still broken.

TODO:

- Test compatibility w/ commercial software.
- Better message validation.
- Tighten error handling, e.g., during corrupt messages.
- State machine isn't complete - it misses some transitions.
- UID string handling (padding \0 at the end).
- Complete the C-STORE client-side impl, and cleanup the code layering structure.
- Remove the "limit" param from the Decoder, and rely on io.EOF detection instead.
