#include <libavcodec/avcodec.h>
#include "video_utils.h"

YUVColor yuv_color(uint8_t y, uint8_t u, uint8_t v) {
    YUVColor result = {.y=y, .u=u, .v=v};
    return result;
}

static void set_pixel(AVPicture* picture, YUVColor yuv_color, int x, int y) {
    picture->data[0][ (x) + (y)*picture->linesize[0] ] = yuv_color.y;
    picture->data[1][ ((x/2) + (y/2)*picture->linesize[1]) ] = yuv_color.u;
    picture->data[2][ ((x/2) + (y/2)*picture->linesize[2]) ] = yuv_color.v;
}

void draw_box(AVPicture *picture, uint32_t x, uint32_t y, uint32_t width, uint32_t height, YUVColor yuv_color)
{
   int i, j, cx,cy;

   for (j = 0; (j < height); j++)
     for (i = 0; (i < width); i++)
       {
         int cx = i+x;
         int cy = y+j;
         set_pixel(picture, yuv_color, cx, cy);
       }
}