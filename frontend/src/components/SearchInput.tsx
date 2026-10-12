import SearchIcon from '@mui/icons-material/Search';
import { InputAdornment, TextField } from '@mui/material';
import { useRef, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';

const SearchInput = () => {
  const ref = useRef<HTMLInputElement>(null);
  const navigate = useNavigate();

  const handleSubmit = (event: FormEvent) => {
    event.preventDefault();
    const name = ref.current?.value.trim();
    if (name) navigate(`/shops?name=${encodeURIComponent(name)}`);
  };

  return (
    <form onSubmit={handleSubmit} style={{ flexGrow: 1, maxWidth: 420 }}>
      <TextField
        inputRef={ref}
        size="small"
        fullWidth
        placeholder="Search shops by name…"
        sx={{ bgcolor: 'white', borderRadius: 1 }}
        slotProps={{
          input: {
            startAdornment: (
              <InputAdornment position="start">
                <SearchIcon fontSize="small" />
              </InputAdornment>
            ),
          },
        }}
      />
    </form>
  );
};

export default SearchInput;
