import { Stack, Typography } from "@mui/material";

interface CompactStatProps {
  label: string;
  value: string;
}

const CompactStat: React.FC<CompactStatProps> = ({ label, value }) => (
  <Stack direction="row" spacing={0.75} sx={{ alignItems: "baseline" }}>
    <Typography variant="body2" color="text.secondary">
      {label}
    </Typography>
    <Typography variant="body2" sx={{ fontWeight: 600 }}>
      {value}
    </Typography>
  </Stack>
);

export default CompactStat;
