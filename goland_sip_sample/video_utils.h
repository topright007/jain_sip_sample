#ifndef _TOPRIGHT_ENCODER_
#define _TOPRIGHT_ENCODER_ 1

#include <libavcodec/avcodec.h>

//const uint8_t YUV_COLOR_GRAY[3] = {(125), (0), (0)};
//const uint8_t YUV_COLOR_BLACK[3] = {(0), (0), (0)};
//const uint8_t YUV_COLOR_WHITE[3] = {(255), (0), (0)};

//#define SET_PIXEL(picture, yuv_color, x, y) { \
//    picture->data[0][ (x) + (y)*picture->linesize[0] ] = yuv_color.y; \
//    picture->data[1][ ((x/2) + (y/2)*picture->linesize[1]) ] = yuv_color.u; \
//    picture->data[2][ ((x/2) + (y/2)*picture->linesize[2]) ] = yuv_color.v; \
//}

typedef struct YUVColor {
    uint8_t y;
    uint8_t u;
    uint8_t v;
} YUVColor;

YUVColor yuv_color(uint8_t y, uint8_t u, uint8_t v);

void draw_box(AVPicture *picture, uint32_t x, uint32_t y, uint32_t width, uint32_t height, YUVColor yuv_color);

uint8_t** wrapWithArray(uint8_t* imagePlane);
void freeWrappedArray(uint8_t** imagePlane);

#endif //_TOPRIGHT_SAMPLE_FFMPEG_ENCODER_