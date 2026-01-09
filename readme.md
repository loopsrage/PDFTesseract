#####
# This is an example to showcase how to build a statically linked binary
# of your go project which uses gosseract.
# The below steps only support the use of PNG files, read the instruction
# on each step to use other formats.
#####

FROM ubuntu:latest AS build-stage

# Build tools and dependencies

# To use other file types, you need to include their library.
# Example: PNG requires libpng-dev and xz-utils.

# To use other languages than English, add their traineddata below.
RUN apt-get update
RUN apt-get install -y \
g++-10 autoconf make git golang libtool pkg-config \
xz-utils libpng-dev \
tesseract-ocr-eng


# Leptonica static build ( required for tesseract )

# To use other file types, you need to remove the disable flag
# for that filetype. Example: remove '--without-jpeg' to use JPEG images.
WORKDIR /build/leptonica
RUN git clone --depth 1 https://github.com/DanBloomberg/leptonica.git .
RUN ./autogen.sh
RUN ./configure '--with-pic' '--disable-shared' '--without-zlib' '--without-libtiff' '--without-giflib' '--without-libwebp' '--without-libwebpmux' '--without-libopenjpeg' '--disable-programs' 'CXX=g++-10' 'CFLAGS=-D DEFAULT_SEVERITY=L_SEVERITY_ERROR -g0 -O3'
RUN make
RUN make install


# Tesseract static build
WORKDIR /build/tesseract
RUN git clone --depth 1 https://github.com/tesseract-ocr/tesseract.git .
RUN ./autogen.sh
RUN ./configure '--with-pic' '--disable-shared' '--disable-legacy' '--disable-graphics' '--disable-openmp' '--without-curl' '--without-archive' '--disable-doc' 'CXX=g++-10' 'CXXFLAGS=-DTESS_EXPORTS -g0 -O3 -ffast-math'
RUN make
RUN make install


# App static build
WORKDIR /build/app
COPY . .

# To user other file types, add their library flags.
# Example: PNG requires -lpng and -lz.
RUN CGO_ENABLED=1 GOOS=linux \
go build  -a -tags netgo -ldflags '-extldflags "-static -L/usr/local/lib -ltesseract -lleptonica -lpng -lz"' ./app.go

[//]: # -ldflags "-extldflags \"-L/usr/local/lib -L/usr/lib/x86_64-linux-gnu -ltesseract -lleptonica -lpng -lz -lcurl -larchive -lstdc++ -lgomp -lpthread\""()

FROM scratch

# Binaries
COPY --from=build-stage /build/app/app /app

# Tesseract traineddata

# To use other language, include their traineddata below
COPY --from=build-stage /usr/share/tesseract-ocr/5/tessdata/eng.traineddata /usr/local/share/tessdata/eng.traineddata


CMD ["/app"]

# Statically linked go build

[Gosseract](https://github.com/otiai10/gosseract) uses the native C api to use tesseract. To build a statically linked go binary, you have to first build both tesseract and leptonica statically. Checkout the [Dockerfile.static](https://github.com/otiai10/gosseract/blob/main/Dockerfile.static) file for step-by-step instructions. The same steps can also be followed in Ubuntu.