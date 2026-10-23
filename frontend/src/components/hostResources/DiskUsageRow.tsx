import { Box, LinearProgress, Typography } from "@mui/material";
import { chartFrameSx } from "./chartShared";

interface DiskUsageRowProps {
  label: string;
  value: string;
  progress: number;
}

const DiskUsageRow: React.FC<DiskUsageRowProps> = ({
  label,
  value,
  progress,
}) => (
  <Box>
    <Box
      sx={{
        display: "flex",
        justifyContent: "space-between",
        alignItems: "baseline",
        mb: 0.5,
        gap: 1,
      }}
    >
      <Typography variant="body2" color="text.secondary">
        {label}
      </Typography>
      <Typography variant="body2" sx={{ fontWeight: 500 }}>
        {value}
      </Typography>
    </Box>
    <Box sx={{ ...chartFrameSx, px: 1.5, py: 1.25 }}>
      <LinearProgress
        variant="determinate"
        value={progress}
        sx={{ height: 6, borderRadius: 1 }}
      />
    </Box>
  </Box>
);

export default DiskUsageRow;
