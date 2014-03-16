package networktables

const sequenceNumberDividingPoint = 32768

type sequenceNumber uint16

func (s sequenceNumber) gt(other sequenceNumber) bool {
	return (s < other && other-s < sequenceNumberDividingPoint) ||
		(s > other && s-other > sequenceNumberDividingPoint)
}
