package image

// operatorImage is the image of the operator, it be set by the linker when building the operator
var operatorImage string

func GetOperatorImage() string {
	if operatorImage == "" {
		return "quay.io/opstree/redis-operator:latest"
	}
	return operatorImage
}
