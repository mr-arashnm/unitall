// API client for the Unital gateway, with graceful demo-mode fallback:
// if the backend is unreachable or the account can't be used, the app
// continues with local demo data so the UI stays reviewable.

const API_BASE = localStorage.getItem("unital_api") ?? "/api/v1";

// ============================================================
// Types — match the OpenAPI schemas in backend/openapi/unital-v1.yaml
// ============================================================

export type ChargeStatus = "paid" | "partially_paid" | "pending" | "overdue" | "cancelled";

export interface Charge {
  id: string;
  unit_id: string;
  template_id: string;
  template_name: string;
  period: string;
  amount: number;
  paid: number;
  remaining: number;
  status: ChargeStatus;
  due_date: string;
}

export interface ChargeTemplate {
  id: string;
  name: string;
  type: "monthly" | "maintenance" | "elevator" | "cleaning" | "security" | "green_space" | "pool" | "gym" | "other";
  amount: number;
  description?: string;
  is_active: boolean;
}

export interface Announcement {
  id: string;
  title: string;
  body?: string;
  content?: string;
  priority: "low" | "normal" | "high" | "urgent";
  status: "draft" | "published";
  date?: string;
  created_at?: string;
}

export interface InboxItem {
  id: string;
  kind: "announcement" | "charge" | "meeting" | "booking";
  title: string;
  body: string;
  date: string;
  read?: boolean;
  meeting?: { going?: boolean };
}

export interface User {
  id: string;
  full_name: string;
  email: string;
  phone?: string;
  national_code?: string;
  platform_role: string;
}

export interface Membership {
  id: string;
  user_id: string;
  building_id: string;
  role: "manager" | "board_member" | "staff" | "owner" | "resident";
  from: string;
}

export interface Building {
  id: string;
  name: string;
  code: string;
  type: "residential" | "commercial" | "office" | "mixed";
  address: string;
  floors?: number;
  features?: string[];
  created_at?: string;
}

export interface Unit {
  id: string;
  building_id: string;
  floor: number;
  number: string;
  area_m2?: number;
  rooms?: number;
  status: "occupied" | "vacant" | "under_construction";
  owner_id?: string;
  resident_id?: string;
  created_at?: string;
}

export interface Asset {
  id: string;
  kind: "parking" | "warehouse";
  code: string;
  building_id: string;
  unit_id?: string;
  floor?: number;
  area_m2?: number;
  available: boolean;
}

export interface UnitParty {
  id: string;
  unit_id: string;
  role: "owner" | "resident";
  user_id: string;
  from: string;
  to?: string;
}

export interface TransferRecord {
  id: string;
  unit_id: string;
  role: "owner" | "resident";
  previous_user_id: string;
  new_user_id: string;
  effective_date: string;
  contract_number?: string;
  recorded_by?: string;
  description?: string;
  created_at?: string;
}

export interface Contract {
  id: string;
  number: string;
  type: "purchase" | "rental" | "transfer";
  unit_id: string;
  first_party_id: string;
  second_party_id: string;
  title?: string;
  amount?: number;
  deposit_amount?: number;
  start_date: string;
  end_date?: string;
  duration_months?: number;
  status: "draft" | "active" | "expired" | "cancelled";
  first_signed: boolean;
  second_signed: boolean;
  signed_date?: string;
  created_by: string;
  created_at?: string;
  updated_at?: string;
}

export interface Facility {
  id: string;
  building_id: string;
  name: string;
  type: string;
  description?: string;
  capacity: number;
  opening_time: string;
  closing_time: string;
  hourly_rate: number;
  min_advance_hours?: number;
  max_advance_hours?: number;
  rules?: string[];
  images?: string[];
  is_active: boolean;
}

export interface Booking {
  id: string;
  facility_id: string;
  building_id: string;
  user_id: string;
  start: string;
  end: string;
  purpose?: string;
  participants?: number;
  status: "pending" | "confirmed" | "completed" | "cancelled" | "rejected";
  total_cost: number;
  approved_by?: string;
  created_at?: string;
}

export interface Team {
  id: string;
  building_id: string;
  name: string;
  type?: string;
  description?: string;
  members: string[];
  is_active: boolean;
}

export interface Task {
  id: string;
  team_id: string;
  building_id: string;
  title: string;
  description?: string;
  priority: "low" | "medium" | "high" | "urgent";
  status: "pending" | "assigned" | "in_progress" | "completed" | "cancelled" | "on_hold";
  assigned_to?: string;
  due_date?: string;
  estimated_hours?: number;
  actual_hours?: number;
  created_at?: string;
}

export interface ServiceRequest {
  id: string;
  building_id: string;
  unit_id: string;
  submitted_by: string;
  title: string;
  description?: string;
  type: "maintenance" | "cleaning" | "security" | "complaint" | "suggestion" | "other";
  priority: "low" | "medium" | "high" | "urgent";
  status: "submitted" | "under_review" | "assigned" | "in_progress" | "completed" | "cancelled";
  assigned_team?: string;
  related_task?: string;
  submitted_at?: string;
  completed_at?: string;
}

export interface Meeting {
  id: string;
  building_id: string;
  title: string;
  type: "board" | "general" | "committee" | "emergency";
  description?: string;
  agenda?: string;
  scheduled_at: string;
  location?: string;
  duration_min: number;
  status: "scheduled" | "ongoing" | "completed" | "cancelled";
  created_by: string;
}

export interface Ticket {
  id: string;
  building_id: string;
  unit_id: string;
  submitted_by: string;
  title: string;
  description?: string;
  type: "technical" | "financial" | "complaint" | "suggestion" | "general";
  priority: "low" | "medium" | "high" | "urgent";
  status: "open" | "in_progress" | "resolved" | "closed";
  assigned_to?: string;
  submitted_at?: string;
}

export interface FinancialReport {
  period: string;
  units: number;
  total_billed: number;
  total_collected: number;
  outstanding: number;
  overdue_count: number;
  collection_rate: number;
}

export interface NotificationTemplate {
  name: string;
  severity: "normal" | "urgent";
  channels: ("inapp" | "email" | "sms" | "webhook")[];
  variants: Record<string, { title: string; body: string }>;
}

// ============================================================
// Session / storage
// ============================================================

const store = {
  get token() { return localStorage.getItem("unital_token"); },
  set token(v: string | null) {
    if (v) localStorage.setItem("unital_token", v);
    else localStorage.removeItem("unital_token");
  },
  get user(): User | null {
    try { return JSON.parse(localStorage.getItem("unital_user") ?? "null"); }
    catch { return null; }
  },
  set user(v: User | null) {
    if (v) localStorage.setItem("unital_user", JSON.stringify(v));
    else localStorage.removeItem("unital_user");
  },
  get currentBuilding(): string | null { return localStorage.getItem("unital_building"); },
  set currentBuilding(v: string | null) {
    if (v) localStorage.setItem("unital_building", v);
    else localStorage.removeItem("unital_building");
  },
  get demo() { return localStorage.getItem("unital_demo") === "1"; },
  set demo(v: boolean) {
    if (v) localStorage.setItem("unital_demo", "1");
    else localStorage.removeItem("unital_demo");
  },
};

export const session = store;

// ============================================================
// HTTP layer
// ============================================================

async function api<T>(path: string, opts: RequestInit = {}): Promise<T> {
  const res = await fetch(API_BASE + path, {
    ...opts,
    headers: {
      "Content-Type": "application/json",
      ...(store.token ? { Authorization: `Bearer ${store.token}` } : {}),
      ...opts.headers,
    },
  });
  if (res.status === 401) { store.token = null; }
  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    const code = (body && typeof body === "object" && "code" in body) ? (body as { code?: string }).code : undefined;
    throw Object.assign(new Error(body?.title ?? body?.detail ?? res.statusText), { body, code, status: res.status });
  }
  return body as T;
}

async function tryApi<T>(path: string, opts?: RequestInit): Promise<T | null> {
  // If we have a live token, always hit the real backend. The previous
  // version short-circuited when `store.demo` was true and silently
  // returned demo data — that broke every action when the user had
  // ever logged in offline. We now require an explicit auth failure to
  // force demo mode.
  if (!store.token) return null;
  return await api<T>(path, opts);
}

// ============================================================
// Auth
// ============================================================

export interface AuthResult { mode: "live" | "demo"; user: User; reason?: string; }

export async function login(email: string, password: string): Promise<AuthResult> {
  // Live mode: hit the real backend. 4xx errors are real and must surface
  // to the UI. We no longer silently invent a demo user on network
  // failure — the user explicitly asked for the app to talk to the
  // backend, not pretend to.
  const pair = await api<{ access_token: string }>("/auth/login", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
  store.token = pair.access_token;
  store.demo = false;
  const me = await api<User>("/me");
  store.user = me;
  return { mode: "live", user: me };
}

export async function register(input: { email: string; password: string; full_name: string; phone?: string }): Promise<AuthResult> {
  await api("/auth/register", { method: "POST", body: JSON.stringify(input) });
  return await login(input.email, input.password);
}

export function logout() {
  store.token = null;
  store.user = null;
  store.demo = false;
  store.currentBuilding = null;
}

export async function updateProfile(patch: Partial<User>): Promise<User> {
  // Live write — must hit the real backend. No local fallback.
  const updated = await api<User>("/me", { method: "PATCH", body: JSON.stringify(patch) });
  store.user = updated;
  return updated;
}

// ============================================================
// Buildings
// ============================================================

// fetchMyBuildings lists the buildings the caller manages or is a party
// of. The gateway routes /me/buildings to identity (memberships only),
// while /buildings on property returns full building objects filtered
// by the caller (created by them, or they hold a party row on a unit).
// We use /buildings so the switcher gets the full objects it needs.
export async function fetchMyBuildings(): Promise<{ mode: "live"; data: Building[] }> {
  const live = await tryApi<{ data: Building[] }>("/buildings").then((r) => r ? { data: r.data.map(asBuilding) } : null);
  if (live) return { mode: "live", data: live.data };
  return { mode: "live", data: [] };
}

export async function fetchBuildings(): Promise<{ mode: "live"; data: Building[] }> {
  const live = await tryApi<{ data: Building[] }>("/buildings").then((r) => r ? { data: r.data.map(asBuilding) } : null);
  if (live) return { mode: "live", data: live.data };
  return { mode: "live", data: [] };
}

export async function fetchBuilding(id: string): Promise<Building> {
  const live = await tryApi<Building>(`/buildings/${id}`);
  if (live) return asBuilding(live);
  throw new Error("Building not found");
}

export async function createBuilding(input: { name: string; code: string; type: Building["type"]; address: string; floors?: number; features?: string[] }): Promise<Building> {
  // Live write to the real backend.
  const live = await tryApi<Building>("/buildings", { method: "POST", body: JSON.stringify(input) });
  if (live) return asBuilding(live);
  throw new Error("Could not create building. Are you signed in?");
}

export async function patchBuilding(id: string, patch: Partial<{ name: string; type: Building["type"]; address: string; floors: number; features: string[] }>): Promise<Building> {
  const live = await tryApi<Building>(`/buildings/${id}`, { method: "PATCH", body: JSON.stringify(patch) });
  if (live) return asBuilding(live);
  throw new Error("Could not update building");
}

// ============================================================
// Units
// ============================================================

export async function createUnit(buildingId: string, input: { floor: number; number: string; area_m2?: number; rooms?: number }): Promise<Unit> {
  const live = await tryApi<Unit>(`/buildings/${buildingId}/units`, { method: "POST", body: JSON.stringify(input) });
  if (live) return live;
  throw new Error("Could not create unit");
}

export async function fetchUnit(unitId: string): Promise<Unit> {
  const live = await tryApi<Unit>(`/units/${unitId}`);
  if (live) return live;
  throw new Error("Unit not found");
}

export async function patchUnit(unitId: string, patch: Partial<{ area_m2: number; rooms: number; status: Unit["status"] }>): Promise<Unit> {
  const live = await tryApi<Unit>(`/units/${unitId}`, { method: "PATCH", body: JSON.stringify(patch) });
  if (live) return live;
  throw new Error("Could not update unit");
}

// ============================================================
// Assets
// ============================================================

export async function createAsset(buildingId: string, input: { kind: "parking" | "warehouse"; code: string; floor: number; area_m2?: number }): Promise<Asset> {
  const live = await tryApi<Asset>(`/buildings/${buildingId}/assets`, { method: "POST", body: JSON.stringify(input) });
  if (live) return live;
  throw new Error("Could not create asset");
}

export async function fetchAsset(assetId: string): Promise<Asset> {
  const live = await tryApi<Asset>(`/assets/${assetId}`);
  if (live) return live;
  throw new Error("Asset not found");
}

export async function patchAsset(assetId: string, patch: Partial<{ code: string; floor: number; area_m2: number }>): Promise<Asset> {
  const live = await tryApi<Asset>(`/assets/${assetId}`, { method: "PATCH", body: JSON.stringify(patch) });
  if (live) return live;
  throw new Error("Could not update asset");
}

export async function assignAsset(unitId: string, kind: "parking" | "warehouse", code: string): Promise<Asset> {
  const live = await tryApi<Asset>(`/units/${unitId}/assets`, { method: "POST", body: JSON.stringify({ kind, code }) });
  if (live) return live;
  throw new Error("Could not assign asset");
}

export async function releaseAsset(unitId: string, kind: "parking" | "warehouse", code: string): Promise<void> {
  await tryApi<{ status: string }>(`/units/${unitId}/assets/${encodeURIComponent(code)}?kind=${kind}`, { method: "DELETE" });
}

// ============================================================
// Parties & Transfers
// ============================================================

export interface UnitPartiesResponse {
  current: { owner?: { user_id: string; from: string }; resident?: { user_id: string; from: string } };
  history: UnitParty[];
}

export async function fetchUnitParties(unitId: string): Promise<UnitPartiesResponse> {
  const live = await tryApi<UnitPartiesResponse>(`/units/${unitId}/parties`);
  if (live) return live;
  return { current: {}, history: [] };
}

export async function fetchUnitTransfers(unitId: string): Promise<TransferRecord[]> {
  const live = await tryApi<{ data: TransferRecord[] }>(`/units/${unitId}/transfer-history`);
  if (live) return live.data || [];
  return [];
}

export async function changeOwnership(unitId: string, input: { new_user_id: string; effective_date: string; contract_number?: string; description?: string }): Promise<TransferRecord> {
  const live = await tryApi<TransferRecord>(`/units/${unitId}/ownership-changes`, { method: "POST", body: JSON.stringify(input) });
  if (live) return live;
  throw new Error("Could not change ownership");
}

export async function changeResidency(unitId: string, input: { new_user_id: string; effective_date: string; contract_number?: string; description?: string }): Promise<TransferRecord> {
  const live = await tryApi<TransferRecord>(`/units/${unitId}/residency-changes`, { method: "POST", body: JSON.stringify(input) });
  if (live) return live;
  throw new Error("Could not change residency");
}

// ============================================================
// Contracts
// ============================================================

export async function createContract(input: { type: Contract["type"]; unit_id: string; first_party_id: string; second_party_id: string; title?: string; amount?: number; deposit_amount?: number; start_date: string; duration_months?: number }): Promise<Contract> {
  const live = await tryApi<Contract>(`/contracts`, { method: "POST", body: JSON.stringify(input) });
  if (live) return live;
  throw new Error("Could not create contract");
}

export async function fetchContract(id: string): Promise<Contract> {
  const live = await tryApi<Contract>(`/contracts/${id}`);
  if (live) return live;
  throw new Error("Contract not found");
}

export async function fetchUnitContracts(unitId: string): Promise<Contract[]> {
  const live = await tryApi<{ data: Contract[] }>(`/units/${unitId}/contracts`);
  if (live) return live.data || [];
  return [];
}

export async function signContract(id: string): Promise<void> {
  await tryApi<{ status: string }>(`/contracts/${id}/sign`, { method: "POST" });
}

export async function activateContract(id: string): Promise<Contract> {
  const live = await tryApi<Contract>(`/contracts/${id}/activate`, { method: "POST" });
  if (live) return live;
  throw new Error("Could not activate contract");
}

// ============================================================
// User search (for party assignment)
// ============================================================

export async function searchUsers(prefix: string): Promise<User[]> {
  const live = await tryApi<{ data: User[] }>(`/internal/users?email=${encodeURIComponent(prefix)}`);
  if (live) return live.data || [];
  return [];
}

export async function inviteUser(email: string): Promise<User> {
  // The gateway forwards this to identity over the internal token. We
  // POST to /internal/users/invite through the same gateway path.
  const live = await tryApi<User>(`/internal/users/invite`, { method: "POST", body: JSON.stringify({ email }) });
  if (live) return live;
  throw new Error("Could not invite user");
}

// ============================================================
// Units & Assets
// ============================================================

export async function fetchUnits(buildingId: string): Promise<Unit[]> {
  const live = await tryApi<{ data: Unit[] }>(`/buildings/${buildingId}/units`);
  if (live) return live.data;
  return [];
}

export async function fetchAssets(buildingId: string, kind: "parking" | "warehouse"): Promise<Asset[]> {
  const live = await tryApi<{ data: Asset[] }>(`/buildings/${buildingId}/assets?kind=${kind}`);
  if (live) return live.data;
  return [];
}

// ============================================================
// Memberships
// ============================================================

export async function fetchMemberships(buildingId: string): Promise<Membership[]> {
  const live = await tryApi<{ data: Membership[] }>(`/buildings/${buildingId}/memberships`);
  if (live) return live.data;
  return [];
}

// ============================================================
// Charges
// ============================================================

export async function fetchCharges(): Promise<{ mode: "live"; data: Charge[] }> {
  const bid = store.currentBuilding;
  if (bid) {
    const live = await tryApi<{ data: Charge[] }>(`/buildings/${bid}/charges`);
    if (live) return { mode: "live", data: live.data };
  }
  return { mode: "live", data: [] };
}

export async function fetchChargeTemplates(buildingId: string): Promise<ChargeTemplate[]> {
  const live = await tryApi<{ data?: ChargeTemplate[] }>(`/buildings/${buildingId}/charge-templates`);
  if (live && Array.isArray(live.data)) return live.data;
  return [];
}

export async function generateCharges(buildingId: string, period: string, dueInDays = 14): Promise<{ created: number; period: string }> {
  const live = await tryApi<{ created: number; period: string }>(`/buildings/${buildingId}/charges:generate`, { method: "POST", body: JSON.stringify({ period, due_in_days: dueInDays }) });
  if (live) return live;
  throw new Error("Could not generate charges");
}

export async function payCharge(chargeId: string, amount: number, method: "online" | "cash" | "bank_transfer" | "cheque" | "pos" = "online"): Promise<unknown> {
  // Real write — POST /charges/{id}/payments
  const live = await tryApi<unknown>(`/charges/${chargeId}/payments`, {
    method: "POST",
    body: JSON.stringify({ amount, method }),
  });
  if (live !== null) return live;
  throw new Error("Could not record payment. Please sign in and try again.");
}

export async function fetchFinancialReport(buildingId: string, period: string): Promise<FinancialReport> {
  const live = await tryApi<FinancialReport>(`/buildings/${buildingId}/reports/financial?period=${period}`);
  if (live) return live;
  throw new Error("Could not load financial report");
}

// ============================================================
// Facilities & Bookings
// ============================================================

export async function fetchFacilities(buildingId: string): Promise<Facility[]> {
  const live = await tryApi<{ data: Facility[] }>(`/buildings/${buildingId}/facilities`);
  if (live) return live.data;
  return [];
}

export async function fetchMyBookings(): Promise<Booking[]> {
  const live = await tryApi<{ data: Booking[] }>("/bookings?mine=true");
  if (live) return live.data;
  return [];
}

export async function bookFacility(facilityId: string, input: { start: string; end: string; purpose?: string; participants?: number }): Promise<Booking> {
  // Live write to the real backend.
  const live = await tryApi<Booking>(`/facilities/${facilityId}/bookings`, { method: "POST", body: JSON.stringify(input) });
  if (live) return live;
  throw new Error("Could not create booking. Please sign in and try again.");
}

export async function cancelBooking(id: string): Promise<Booking | null> {
  return await tryApi<Booking>(`/bookings/${id}/cancel`, { method: "POST" });
}

// ============================================================
// Operations (teams, tasks, service requests)
// ============================================================

export async function fetchTeams(buildingId: string): Promise<Team[]> {
  const live = await tryApi<{ data: Team[] }>(`/buildings/${buildingId}/teams`);
  if (live) return live.data;
  return [];
}

export async function fetchTasks(buildingId: string, teamId?: string): Promise<Task[]> {
  const q = teamId ? `?team_id=${teamId}` : "";
  const live = await tryApi<{ data: Task[] }>(`/buildings/${buildingId}/tasks${q}`);
  if (live) return live.data;
  return [];
}

export async function fetchServiceRequests(buildingId: string): Promise<ServiceRequest[]> {
  const live = await tryApi<{ data: ServiceRequest[] }>(`/buildings/${buildingId}/service-requests`);
  if (live) return live.data;
  return [];
}

export async function submitServiceRequest(input: { unit_id: string; title: string; description?: string; type: ServiceRequest["type"]; priority: ServiceRequest["priority"] }): Promise<ServiceRequest> {
  const bid = store.currentBuilding;
  if (!bid) throw new Error("Select a building first");
  // Live write to the real backend.
  const live = await tryApi<ServiceRequest>(`/buildings/${bid}/service-requests`, { method: "POST", body: JSON.stringify(input) });
  if (live) return live;
  throw new Error("Could not submit request. Please sign in and try again.");
}

// ============================================================
// Comms (announcements, meetings, tickets)
// ============================================================

export async function fetchAnnouncements(buildingId: string): Promise<Announcement[]> {
  const live = await tryApi<{ data: Announcement[] }>(`/buildings/${buildingId}/announcements`);
  if (live) return live.data;
  return [];
}

export async function fetchMeetings(buildingId: string): Promise<Meeting[]> {
  const live = await tryApi<{ data: Meeting[] }>(`/buildings/${buildingId}/meetings`);
  if (live) return live.data;
  return [];
}

export async function fetchTickets(buildingId: string): Promise<Ticket[]> {
  const live = await tryApi<{ data: Ticket[] }>(`/buildings/${buildingId}/tickets`);
  if (live) return live.data;
  return [];
}

export async function openTicket(input: { unit_id: string; title: string; description?: string; type: Ticket["type"]; priority: Ticket["priority"] }): Promise<Ticket> {
  const bid = store.currentBuilding;
  if (!bid) throw new Error("Select a building first");
  // Live write to the real backend.
  const live = await tryApi<Ticket>(`/buildings/${bid}/tickets`, { method: "POST", body: JSON.stringify(input) });
  if (live) return live;
  throw new Error("Could not open ticket. Please sign in and try again.");
}

// ============================================================
// Inbox
// ============================================================

export async function fetchInbox(): Promise<{ mode: "live"; data: InboxItem[] }> {
  const live = await tryApi<{ data: InboxItem[] }>("/me/notifications");
  if (live) return { mode: "live", data: live.data };
  return { mode: "live", data: [] };
}

export async function markInboxRead(id: string): Promise<void> {
  await tryApi(`/me/notifications/${id}/read`, { method: "POST" });
}

// ============================================================
// Notifications / templates
// ============================================================

export async function fetchNotificationTemplates(): Promise<NotificationTemplate[]> {
  const live = await tryApi<{ data: NotificationTemplate[] }>("/templates");
  if (live) return live.data;
  return [];
}

// ============================================================
// Formatting helpers
// ============================================================

export const fmtRial = (v: number) => `${new Intl.NumberFormat("en-US").format(v)} Rial`;

export const fmtRialShort = (v: number) => {
  if (v >= 1_000_000_000) return `${(v / 1_000_000_000).toFixed(1)}B Rial`;
  if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(1)}M Rial`;
  if (v >= 1_000) return `${(v / 1_000).toFixed(0)}K Rial`;
  return `${v} Rial`;
};

export const fmtDate = (iso: string) =>
  new Intl.DateTimeFormat("en-US", { day: "numeric", month: "long", year: "numeric" }).format(new Date(iso));

export const fmtDateTime = (iso: string) =>
  new Intl.DateTimeFormat("en-US", { day: "numeric", month: "long", hour: "2-digit", minute: "2-digit" }).format(new Date(iso));

export const fmtPeriod = (p: string) => {
  // YYYY-MM → e.g. "Sep 2026"
  const months = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
  const [y, m] = p.split("-");
  return `${months[Number(m) - 1] ?? ""} ${y}`;
};

export const toEn = (n: number) => new Intl.NumberFormat("en-US").format(n);

// ============================================================
// Helpers
// ============================================================

function asBuilding(b: any): Building {
  return {
    id: b.id, name: b.name, code: b.code, type: b.type, address: b.address,
    floors: b.floors, features: b.features ?? [], created_at: b.created_at,
  };
}

// ============================================================
// Demo data
// ============================================================

function demoBuildings(): Building[] {
  return [
    { id: "b1", name: "Unital Towers", code: "TWR-1", type: "residential", address: "123 Main Street, Downtown", floors: 10, features: ["billing", "notifications", "facilities", "operations"] },
    { id: "b2", name: "Pars Complex", code: "PRS-A", type: "mixed", address: "Tajrish Square, North Side", floors: 6, features: ["billing", "notifications"] },
    { id: "b3", name: "Sky Tower", code: "SKY-3", type: "office", address: "Vanak Blvd, Mellat Street", floors: 18, features: ["billing", "facilities"] },
  ];
}

function demoUnits(buildingId: string): Unit[] {
  return Array.from({ length: 12 }, (_, i) => ({
    id: `u_${buildingId}_${i + 1}`,
    building_id: buildingId,
    floor: Math.floor(i / 3) + 1,
    number: `${Math.floor(i / 3) + 1}0${(i % 3) + 1}`,
    area_m2: 90 + (i % 4) * 15,
    rooms: 2 + (i % 3),
    status: i % 4 === 0 ? "vacant" : i % 5 === 0 ? "under_construction" : "occupied",
  }));
}

function demoAssets(buildingId: string, kind: "parking" | "warehouse"): Asset[] {
  return Array.from({ length: 8 }, (_, i) => ({
    id: `a_${kind}_${i + 1}`,
    kind,
    code: `${kind === "parking" ? "P" : "W"}-${String(i + 1).padStart(2, "0")}`,
    building_id: buildingId,
    floor: kind === "parking" ? -1 : -2,
    area_m2: kind === "parking" ? 12 : 4,
    available: i % 3 !== 0,
  }));
}

function demoMemberships(buildingId: string): Membership[] {
  return [
    { id: "m1", user_id: "u1", building_id: buildingId, role: "manager", from: "2024-01-01" },
    { id: "m2", user_id: "u2", building_id: buildingId, role: "board_member", from: "2024-03-15" },
    { id: "m3", user_id: "u3", building_id: buildingId, role: "owner", from: "2024-02-10" },
    { id: "m4", user_id: "u4", building_id: buildingId, role: "resident", from: "2024-06-01" },
    { id: "m5", user_id: "u5", building_id: buildingId, role: "staff", from: "2024-04-20" },
  ];
}

function demoCharges(): Charge[] {
  return [
    { id: "c1", unit_id: "u_b1_1", template_id: "t1", template_name: "Monthly fee", period: "2026-06", amount: 2_500_000, paid: 2_500_000, remaining: 0, status: "paid", due_date: "2026-08-21" },
    { id: "c2", unit_id: "u_b1_1", template_id: "t2", template_name: "Elevator service", period: "2026-07", amount: 400_000, paid: 0, remaining: 400_000, status: "pending", due_date: "2026-09-10" },
    { id: "c3", unit_id: "u_b1_1", template_id: "t1", template_name: "Monthly fee", period: "2026-05", amount: 2_500_000, paid: 1_000_000, remaining: 1_500_000, status: "overdue", due_date: "2026-07-21" },
  ];
}

function demoChargeTemplates(buildingId: string): ChargeTemplate[] {
  return [
    { id: "t1", name: "Monthly fee", type: "monthly", amount: 2_500_000, is_active: true },
    { id: "t2", name: "Elevator service", type: "elevator", amount: 400_000, is_active: true },
    { id: "t3", name: "Security", type: "security", amount: 300_000, is_active: true },
    { id: "t4", name: "Green space", type: "green_space", amount: 150_000, is_active: true },
  ];
}

function demoFacilities(buildingId: string): Facility[] {
  return [
    { id: "f1", building_id: buildingId, name: "Party Hall", type: "party_hall", capacity: 80, opening_time: "08:00", closing_time: "23:00", hourly_rate: 200_000, rules: ["Please return the hall clean", "No smoking"], is_active: true },
    { id: "f2", building_id: buildingId, name: "Swimming Pool", type: "pool", capacity: 20, opening_time: "06:00", closing_time: "22:00", hourly_rate: 50_000, is_active: true },
    { id: "f3", building_id: buildingId, name: "Fitness Center", type: "gym", capacity: 15, opening_time: "06:00", closing_time: "23:00", hourly_rate: 0, is_active: true },
    { id: "f4", building_id: buildingId, name: "Meeting Room", type: "meeting_room", capacity: 12, opening_time: "09:00", closing_time: "21:00", hourly_rate: 100_000, is_active: true },
    { id: "f5", building_id: buildingId, name: "Guest Parking", type: "guest_parking", capacity: 8, opening_time: "00:00", closing_time: "23:59", hourly_rate: 0, is_active: true },
  ];
}

function demoBookings(): Booking[] {
  return [
    { id: "bk1", facility_id: "f1", building_id: "b1", user_id: "u1", start: "2026-09-05T17:00:00Z", end: "2026-09-05T20:00:00Z", purpose: "Birthday party", participants: 25, status: "confirmed", total_cost: 600_000 },
    { id: "bk2", facility_id: "f4", building_id: "b1", user_id: "u1", start: "2026-09-02T18:00:00Z", end: "2026-09-02T19:30:00Z", purpose: "Board meeting", participants: 8, status: "pending", total_cost: 150_000 },
  ];
}

function demoTeams(buildingId: string): Team[] {
  return [
    { id: "tm1", building_id: buildingId, name: "Security Team", type: "security", members: ["u5", "u6"], is_active: true },
    { id: "tm2", building_id: buildingId, name: "Maintenance Team", type: "maintenance", members: ["u7", "u8"], is_active: true },
    { id: "tm3", building_id: buildingId, name: "Cleaning Team", type: "cleaning", members: ["u9"], is_active: true },
  ];
}

function demoTasks(buildingId: string): Task[] {
  return [
    { id: "tk1", team_id: "tm2", building_id: buildingId, title: "Repair pump on floor 3", description: "Floor 3 water pump is making unusual noise", priority: "high", status: "in_progress", assigned_to: "u7", created_at: "2026-08-22" },
    { id: "tk2", team_id: "tm2", building_id: buildingId, title: "Elevator service", priority: "medium", status: "pending", created_at: "2026-08-20" },
    { id: "tk3", team_id: "tm1", building_id: buildingId, title: "Replace night shift guard", priority: "low", status: "completed", assigned_to: "u5", created_at: "2026-08-15" },
  ];
}

function demoServiceRequests(buildingId: string): ServiceRequest[] {
  return [
    { id: "sr1", building_id: buildingId, unit_id: "u_b1_1", submitted_by: "u1", title: "Kitchen pipe leak", description: "Water dripping from under the kitchen sink", type: "maintenance", priority: "high", status: "in_progress", assigned_team: "tm2", submitted_at: "2026-08-23" },
    { id: "sr2", building_id: buildingId, unit_id: "u_b1_1", submitted_by: "u1", title: "Replace hallway light bulb", type: "maintenance", priority: "low", status: "completed", submitted_at: "2026-08-10", completed_at: "2026-08-12" },
  ];
}

function demoMeetings(buildingId: string): Meeting[] {
  return [
    { id: "mt1", building_id: buildingId, title: "Annual General Assembly", type: "general", description: "Financial report and board election", agenda: "1. Manager's report  2. Financial statements  3. Voting", scheduled_at: "2026-09-15T18:00:00Z", location: "Party Hall", duration_min: 120, status: "scheduled", created_by: "u1" },
    { id: "mt2", building_id: buildingId, title: "Emergency meeting — water outage", type: "emergency", scheduled_at: "2026-08-20T19:00:00Z", location: "Party Hall", duration_min: 60, status: "completed", created_by: "u1" },
  ];
}

function demoTickets(buildingId: string): Ticket[] {
  return [
    { id: "tk_t1", building_id: buildingId, unit_id: "u_b1_1", submitted_by: "u1", title: "Payment system error", description: "Getting 503 error during online checkout", type: "technical", priority: "medium", status: "open", submitted_at: "2026-08-24" },
    { id: "tk_t2", building_id: buildingId, unit_id: "u_b1_1", submitted_by: "u1", title: "Suggestion: install cameras", description: "Would suggest installing cameras in the parking area", type: "suggestion", priority: "low", status: "resolved", submitted_at: "2026-08-15" },
  ];
}

const demoAnnouncements: Announcement[] = [
  { id: "a1", title: "Water outage — Friday", content: "For pump maintenance, the building water will be off from 9 AM to 1 PM.", body: "For pump maintenance, the building water will be off from 9 AM to 1 PM.", priority: "high", status: "published", date: "2026-08-20" },
  { id: "a2", title: "Annual General Assembly", content: "The assembly will be held on Monday at 6 PM in the Party Hall. Attendance is required for some owners.", body: "The assembly will be held on Monday at 6 PM in the Party Hall.", priority: "normal", status: "published", date: "2026-08-18" },
  { id: "a3", title: "Monthly fee increase", content: "Approved by the board, the monthly fee will increase by 10% starting October.", body: "Approved by the board, the monthly fee will increase by 10% starting October.", priority: "urgent", status: "published", date: "2026-08-15" },
];

const demoInbox: InboxItem[] = [
  { id: "i1", kind: "charge", title: "Overdue charge reminder", body: "Your charge for period 2026-05 is overdue.", date: "2026-08-22", read: false },
  { id: "i2", kind: "meeting", title: "General Assembly invitation", body: "Monday at 6 PM — Party Hall", date: "2026-08-18", meeting: { going: undefined } },
  { id: "i3", kind: "booking", title: "Hall booking confirmed", body: "Your booking for Friday at 5 PM is confirmed.", date: "2026-08-15", read: true },
  { id: "i4", kind: "announcement", title: "Water outage", body: "Friday 9 AM to 1 PM — water will be off.", date: "2026-08-20", read: true },
];

const demoTemplates: NotificationTemplate[] = [
  { name: "charge.overdue.reminder", severity: "urgent", channels: ["inapp", "sms"], variants: { inapp: { title: "Overdue charge", body: "Your outstanding balance: {{remaining}} Rial" } } },
  { name: "meeting.invite", severity: "normal", channels: ["inapp", "email"], variants: { inapp: { title: "Meeting invitation", body: "{{title}} at {{location}}" } } },
  { name: "booking.confirmed", severity: "normal", channels: ["inapp"], variants: { inapp: { title: "Booking confirmed", body: "Your booking of {{facility}} on {{date}} is confirmed" } } },
];
