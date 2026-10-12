import { Navigate, useLocation } from 'react-router-dom';

// The shop list moved to "/"; keep old /shops?type_id=... links working.
const ShopsRedirect = () => {
  const location = useLocation();
  return <Navigate to={`/${location.search}`} replace />;
};

export default ShopsRedirect;
