package database

// InitDB initializes the database connection
// func InitDB() error {
// 	// GORM logging config
// 	gormConfig := &gorm.Config{
// 		Logger: logger.Default.LogMode(logger.Info),
// 	}

// 	var err error

// 	switch config.AppConfig.DBDriver {
// 	case "postgres":
// 		// Build DSN from env-based config
// 		dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=UTC",
// 			config.AppConfig.DBHost,
// 			config.AppConfig.DBPort,
// 			config.AppConfig.DBUser,
// 			config.AppConfig.DBPassword,
// 			config.AppConfig.DBName)

// 		log.Printf("🔌 Connecting to PostgreSQL at host=%s port=%s dbname=%s...",
// 			config.AppConfig.DBHost,
// 			config.AppConfig.DBPort,
// 			config.AppConfig.DBName)

// 		DB, err = gorm.Open(postgres.Open(dsn), gormConfig)
// 		if err != nil {
// 			log.Printf("❌ Failed to connect to PostgreSQL: %v", err)
// 			return err
// 		}

// 		log.Println("✅ PostgreSQL connection successful.")

// 	case "sqlite", "sqlite3":
// 		dbDir := filepath.Dir(config.AppConfig.DBPath)
// 		if err := os.MkdirAll(dbDir, os.ModePerm); err != nil {
// 			log.Printf("❌ Failed to create SQLite folder: %v", err)
// 			return err
// 		}

// 		DB, err = gorm.Open(sqlite.Open(config.AppConfig.DBPath), gormConfig)
// 		if err != nil {
// 			log.Printf("❌ Failed to connect to SQLite: %v", err)
// 			return err
// 		}

// 		log.Printf("✅ SQLite connection successful at %s", config.AppConfig.DBPath)

// 	default:
// 		return fmt.Errorf("unsupported DB driver: %s", config.AppConfig.DBDriver)
// 	}

// 	return nil
// }

// CloseDB closes the database connection
func CloseDB() error {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}
