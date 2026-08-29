/* post-processed by ./scripts/process-types.go */
/* eslint-disable */
/* tslint:disable */
/*
 * ---------------------------------------------------------------
 * ## THIS FILE WAS GENERATED VIA SWAGGER-TYPESCRIPT-API        ##
 * ##                                                           ##
 * ## AUTHOR: acacode                                           ##
 * ## SOURCE: https://github.com/acacode/swagger-typescript-api ##
 * ---------------------------------------------------------------
 */

export interface CurrenciesCurrency {
  code: string;
  local: string;
  name: string;
  symbol: string;
}

export interface ProductlookupProduct {
  amount: string;
  brand: string;
  found: boolean;
  name: string;
  source: string;
}

export interface ConsumptionCreate {
  /**
   * Amount is always positive; Type carries the direction.
   * @min 1
   */
  amount: number;
  /** Date defaults to now when zero. */
  date: Date | string;
  /** @maxLength 500 */
  note: string;
  type: "consume" | "restock" | "correction";
}

export interface ConsumptionEntry {
  amount: number;
  createdAt: Date | string;
  date: Date | string;
  id: string;
  itemId: string;
  note: string;
  type: string;
}

export interface ConsumptionSummary {
  /**
   * AveragePerWeek is the consumed amount projected onto a 7 day window
   * over the requested period. Useful to judge how long stock will last.
   */
  averagePerWeek: number;
  entries: number;
  itemId: string;
  itemName: string;
  totalConsumed: number;
  totalRestocked: number;
}

export interface DocumentOut {
  id: string;
  path: string;
  title: string;
}

export interface Group {
  createdAt: Date | string;
  currency: string;
  id: string;
  name: string;
  updatedAt: Date | string;
}

export interface GroupStatistics {
  totalItemPrice: number;
  totalItems: number;
  totalLabels: number;
  totalLocations: number;
  totalUsers: number;
  totalWithWarranty: number;
}

export interface GroupUpdate {
  currency: string;
  name: string;
}

export interface ItemAttachment {
  createdAt: Date | string;
  document: DocumentOut;
  id: string;
  primary: boolean;
  type: string;
  updatedAt: Date | string;
}

export interface ItemAttachmentUpdate {
  primary: boolean;
  title: string;
  type: string;
}

export interface ItemCreate {
  /**
   * Barcode lets an item be created straight from a scan, so the code is
   * registered without a second trip through the edit form.
   * @maxLength 255
   */
  barcode: string;
  /** @maxLength 1000 */
  description: string;
  /**
   * The pantry numbers are settable on create for the same reason: when
   * unpacking a shopping bag, going back into the edit form for every
   * tin is the slowest part of the job.
   */
  expiryDate: Date | string;
  labelIds: string[];
  /** Edges */
  locationId: string;
  minStock: number;
  /**
   * @minLength 1
   * @maxLength 255
   */
  name: string;
  parentId?: string | null;
}

export interface ItemField {
  booleanValue: boolean;
  id: string;
  name: string;
  numberValue: number;
  textValue: string;
  type: string;
}

export interface ItemOut {
  archived: boolean;
  /** @example "0" */
  assetId: string;
  attachments: ItemAttachment[];
  barcode: string;
  createdAt: Date | string;
  description: string;
  /**
   * Pantry - kept on the summary so list views can flag expiring and
   * low-stock items without fetching each item individually.
   */
  expiryDate: Date | string;
  fields: ItemField[];
  id: string;
  imageId: string;
  insured: boolean;
  labels: LabelSummary[];
  /** Warranty */
  lifetimeWarranty: boolean;
  /** Edges */
  location?: LocationSummary | null;
  manufacturer: string;
  minStock: number;
  modelNumber: string;
  name: string;
  /** Extras */
  notes: string;
  parent?: ItemSummary | null;
  purchaseFrom: string;
  purchasePrice: number;
  /** Purchase */
  purchaseTime: Date | string;
  quantity: number;
  serialNumber: string;
  soldNotes: string;
  soldPrice: number;
  /** Sold */
  soldTime: Date | string;
  soldTo: string;
  syncChildItemsLocations: boolean;
  updatedAt: Date | string;
  warrantyDetails: string;
  warrantyExpires: Date | string;
}

export interface ItemPatch {
  id: string;
  quantity?: number | null;
}

export interface ItemPath {
  id: string;
  name: string;
  type: ItemType;
}

export interface ItemSummary {
  archived: boolean;
  /** @example "0" */
  assetId: string;
  barcode: string;
  createdAt: Date | string;
  description: string;
  /**
   * Pantry - kept on the summary so list views can flag expiring and
   * low-stock items without fetching each item individually.
   */
  expiryDate: Date | string;
  id: string;
  imageId: string;
  insured: boolean;
  labels: LabelSummary[];
  /** Edges */
  location?: LocationSummary | null;
  minStock: number;
  name: string;
  purchasePrice: number;
  quantity: number;
  updatedAt: Date | string;
}

export enum ItemType {
  ItemTypeLocation = "location",
  ItemTypeItem = "item",
}

export interface ItemUpdate {
  archived: boolean;
  assetId: string;
  /** @maxLength 255 */
  barcode: string;
  /** @maxLength 1000 */
  description: string;
  /** Pantry */
  expiryDate: Date | string;
  fields: ItemField[];
  id: string;
  insured: boolean;
  labelIds: string[];
  /** Warranty */
  lifetimeWarranty: boolean;
  /** Edges */
  locationId: string;
  manufacturer: string;
  minStock: number;
  modelNumber: string;
  /**
   * @minLength 1
   * @maxLength 255
   */
  name: string;
  /** Extras */
  notes: string;
  parentId?: string | null;
  /** @maxLength 255 */
  purchaseFrom: string;
  purchasePrice?: number | null;
  /** Purchase */
  purchaseTime: Date | string;
  quantity: number;
  /** Identifications */
  serialNumber: string;
  soldNotes: string;
  soldPrice?: number | null;
  /** Sold */
  soldTime: Date | string;
  /** @maxLength 255 */
  soldTo: string;
  syncChildItemsLocations: boolean;
  warrantyDetails: string;
  warrantyExpires: Date | string;
}

export interface LabelCreate {
  color: string;
  /** @maxLength 255 */
  description: string;
  /**
   * @minLength 1
   * @maxLength 255
   */
  name: string;
}

export interface LabelOut {
  createdAt: Date | string;
  description: string;
  id: string;
  name: string;
  updatedAt: Date | string;
}

export interface LabelSummary {
  createdAt: Date | string;
  description: string;
  id: string;
  name: string;
  updatedAt: Date | string;
}

export interface LocationCreate {
  description: string;
  name: string;
  parentId?: string | null;
}

export interface LocationOut {
  children: LocationSummary[];
  createdAt: Date | string;
  description: string;
  id: string;
  name: string;
  parent: LocationSummary;
  totalPrice: number;
  updatedAt: Date | string;
}

export interface LocationOutCount {
  createdAt: Date | string;
  description: string;
  id: string;
  itemCount: number;
  name: string;
  updatedAt: Date | string;
}

export interface LocationSummary {
  createdAt: Date | string;
  description: string;
  id: string;
  name: string;
  updatedAt: Date | string;
}

export interface LocationUpdate {
  description: string;
  id: string;
  name: string;
  parentId?: string | null;
}

export interface MaintenanceEntry {
  completedDate: Date | string;
  /** @example "0" */
  cost: string;
  description: string;
  id: string;
  name: string;
  scheduledDate: Date | string;
}

export interface MaintenanceEntryCreate {
  completedDate: Date | string;
  /** @example "0" */
  cost: string;
  description: string;
  name: string;
  scheduledDate: Date | string;
}

export interface MaintenanceEntryUpdate {
  completedDate: Date | string;
  /** @example "0" */
  cost: string;
  description: string;
  name: string;
  scheduledDate: Date | string;
}

export interface MaintenanceEntryWithDetails {
  completedDate: Date | string;
  /** @example "0" */
  cost: string;
  description: string;
  id: string;
  itemID: string;
  itemName: string;
  name: string;
  scheduledDate: Date | string;
}

export enum MaintenanceFilterStatus {
  MaintenanceFilterStatusScheduled = "scheduled",
  MaintenanceFilterStatusCompleted = "completed",
  MaintenanceFilterStatusBoth = "both",
}

export interface NotifierCreate {
  isActive: boolean;
  /**
   * @minLength 1
   * @maxLength 255
   */
  name: string;
  url: string;
}

export interface NotifierOut {
  createdAt: Date | string;
  groupId: string;
  id: string;
  isActive: boolean;
  name: string;
  updatedAt: Date | string;
  url: string;
  userId: string;
}

export interface NotifierUpdate {
  isActive: boolean;
  /**
   * @minLength 1
   * @maxLength 255
   */
  name: string;
  url?: string | null;
}

export interface PaginationResultItemSummary {
  items: ItemSummary[];
  page: number;
  pageSize: number;
  total: number;
}

export interface TotalsByOrganizer {
  id: string;
  name: string;
  total: number;
}

export interface TreeItem {
  children: TreeItem[];
  id: string;
  name: string;
  type: string;
}

export interface UserOut {
  email: string;
  groupId: string;
  groupName: string;
  id: string;
  isOwner: boolean;
  isSuperuser: boolean;
  name: string;
}

export interface UserUpdate {
  email: string;
  name: string;
}

export interface ValueOverTime {
  end: string;
  entries: ValueOverTimeEntry[];
  start: string;
  valueAtEnd: number;
  valueAtStart: number;
}

export interface ValueOverTimeEntry {
  date: Date | string;
  name: string;
  value: number;
}

export interface Latest {
  date: Date | string;
  version: string;
}

export interface UserRegistration {
  email: string;
  name: string;
  password: string;
  token: string;
}

export interface APISummary {
  allowRegistration: boolean;
  build: Build;
  demo: boolean;
  health: boolean;
  labelPrinting: boolean;
  latest: Latest;
  message: string;
  productLookup: boolean;
  title: string;
  versions: string[];
}

export interface ActionAmountResult {
  completed: number;
}

export interface AdminUserCreate {
  email: string;
  groupID: string;
  isSuperuser: boolean;
  name: string;
  password: string;
}

export interface Build {
  buildTime: string;
  commit: string;
  version: string;
}

export interface ChangePassword {
  current: string;
  new: string;
}

export interface GroupInvitation {
  expiresAt: Date | string;
  token: string;
  uses: number;
}

export interface GroupInvitationCreate {
  expiresAt: Date | string;
  /**
   * @min 1
   * @max 100
   */
  uses: number;
}

export interface ItemAttachmentToken {
  token: string;
}

export interface LoginForm {
  /** @example "admin" */
  password: string;
  stayLoggedIn: boolean;
  /** @example "admin@admin.com" */
  username: string;
}

export interface ResultsRepoUserOut {
  items: UserOut[];
}

export interface ScanResult {
  barcode: string;
  items: ItemSummary[];
  /**
   * Suggestion is nil unless the barcode is unknown locally and the
   * lookup is enabled and produced something.
   */
  suggestion?: ProductlookupProduct | null;
}

export interface TokenResponse {
  attachmentToken: string;
  expiresAt: Date | string;
  token: string;
}

export interface Wrapped {
  item: any;
}

export interface ValidateErrorResponse {
  error: string;
  fields: string;
}
