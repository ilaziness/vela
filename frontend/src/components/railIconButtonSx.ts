import type { SxProps, Theme } from "@mui/material/styles";

/**
 * 左侧栏图标按钮。不用 IconButton 的 size：size 的 fontSize 经 pxToRem，会随应用字号缩放。
 * 图标默认也走 pxToRem，这里一并锁成像素。
 */
export const railIconButtonSx: SxProps<Theme> = {
  padding: "5px",
  fontSize: "18px",
  "&& .MuiSvgIcon-root": {
    fontSize: "20px",
    width: "20px",
    height: "20px",
  },
};
