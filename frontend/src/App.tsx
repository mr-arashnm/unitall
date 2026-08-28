import { BrowserRouter, Navigate, Outlet, Route, Routes } from "react-router-dom";
import Layout from "./components/Layout";
import { ErrorBoundary } from "./components/ErrorBoundary";
import Login from "./screens/Login";
import Home from "./screens/Home";
import Charges from "./screens/Charges";
import Requests from "./screens/Requests";
import Inbox from "./screens/Inbox";
import Unit from "./screens/Unit";
import Buildings from "./screens/Buildings";
import BuildingDetail from "./screens/BuildingDetail";
import NewBuilding from "./screens/NewBuilding";
import UnitDetail from "./screens/UnitDetail";
import AssetDetail from "./screens/AssetDetail";
import Facilities from "./screens/Facilities";
import Teams from "./screens/Teams";
import Announcements from "./screens/Announcements";
import Meetings from "./screens/Meetings";
import Tickets from "./screens/Tickets";
import Reports from "./screens/Reports";
import Profile from "./screens/Profile";
import { session } from "./api";

// AuthedShell wraps the entire authenticated app using React Router v6
// nested routes via <Outlet>. Layout mounts once on first auth and
// stays alive across page navigations, so its useEffects (buildings
// list, etc.) fire only once instead of on every route change.
// Before this fix: each <Route element={<Layout>{Page}</Layout>}> created
// a new Layout instance per navigation, re-fetching buildings every time.
function AuthedShell() {
  if (!session.user) return <Navigate to="/login" replace />;
  return (
    <Layout>
      <ErrorBoundary>
        <Outlet />
      </ErrorBoundary>
    </Layout>
  );
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route element={<AuthedShell />}>
          <Route path="/" element={<Home />} />
          <Route path="buildings" element={<Buildings />} />
          <Route path="buildings/new" element={<NewBuilding />} />
          <Route path="buildings/:id" element={<BuildingDetail />} />
          <Route path="buildings/:id/units/:unitId" element={<UnitDetail />} />
          <Route path="buildings/:id/assets/:assetId" element={<AssetDetail />} />
          <Route path="charges" element={<Charges />} />
          <Route path="requests" element={<Requests />} />
          <Route path="inbox" element={<Inbox />} />
          <Route path="unit" element={<Unit />} />
          <Route path="facilities" element={<Facilities />} />
          <Route path="teams" element={<Teams />} />
          <Route path="announcements" element={<Announcements />} />
          <Route path="meetings" element={<Meetings />} />
          <Route path="tickets" element={<Tickets />} />
          <Route path="reports" element={<Reports />} />
          <Route path="profile" element={<Profile />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
