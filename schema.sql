CREATE TABLE records (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    upload_time TIMESTAMP DEFAULT NOW() NOT NULL
);

create table sentences(
  id SERIAL PRIMARY KEY,
  sentence TEXT not null,
  pdfId INT not null,
  FOREIGN KEY (pdfId) references records(id) ON DELETE CASCADE
  );