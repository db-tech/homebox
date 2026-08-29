export enum AttachmentTypes {
  Photo = "photo",
  Manual = "manual",
  Warranty = "warranty",
  Attachment = "attachment",
  Receipt = "receipt",
}

export type Result<T> = {
  item: T;
};

export interface PaginationResult<T> {
  items: T[];
  page: number;
  pageSize: number;
  total: number;
}

export interface ItemSummaryPaginationResult<T> extends PaginationResult<T> {
  totalPrice: number;
}

/** A photo staged in the create form, before it is uploaded as an attachment. */
export interface PhotoPreview {
  photoName: string;
  fileBase64: string;
  file: File;
}
