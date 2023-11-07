package router

//import "net"
//
//type Writer struct {
//	conn *net.UDPConn
//	addr *net.UDPAddr
//}
//
//func (w *Writer) Write(p []byte) (n int, err error) {
//	w.conn.
//
//
//
//
//	var addr *net.UDPAddr
//	var buf = make([]byte, len(p))
//
//	for {
//		n, addr, err = w.conn.ReadFromUDP(buf)
//		if err != nil {
//			return 0, err
//		}
//		if addr.String() == w.addr.String() {
//			break
//		}
//	}
//
//	return copy(p, buf), nil
//}
//
//func (w *Writer) Close() error {
//	return w.conn.Close()
//}
