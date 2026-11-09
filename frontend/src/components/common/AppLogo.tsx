import { Box } from "@mui/material";
import { dragRegionSx } from "../../hooks/useFrameless";

interface AppLogoProps {
  frameless?: boolean;
}

/** 左侧栏顶标：与 30px 按钮同宽、与标签行同高。 */
export default function AppLogo({ frameless = false }: AppLogoProps) {
  return (
    <Box
      sx={{
        width: 30,
        height: 34,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        flexShrink: 0,
        ...dragRegionSx(frameless),
        cursor: frameless ? "move" : undefined,
      }}
    >
      <Box
        component="img"
        src="/appicon.png"
        alt=""
        draggable={false}
        sx={{
          width: 28,
          height: 28,
          display: "block",
          borderRadius: "6px",
        }}
      />
    </Box>
  );
}
